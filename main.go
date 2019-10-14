package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	webm2 "github.com/ebml-go/webm"
	"github.com/jeffallen/seekinghttp"
	"lionPlayer/youtube"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

var token string

var ytsrc = youtube.NewYoutubeSource()

func main() {
	if token == "" {
		fmt.Println("No token provided. Please run: airhorn -t <bot token>")
		return
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// Register ready as a callback for the ready events.
	dg.AddHandler(ready)

	// Register messageCreate as a callback for the messageCreate events.
	dg.AddHandler(messageCreate)

	// Register guildCreate as a callback for the guildCreate events.
	dg.AddHandler(guildCreate)

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Airhorn is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func ready(s *discordgo.Session, event *discordgo.Ready) {

	// Set the playing status.
	s.UpdateStatus(0, "!airhorn")
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// check if the message is "!airhorn"
	if strings.HasPrefix(m.Content, "!play") {
		// Find the channel that the message came from.
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			// Could not find channel.
			return
		}

		splut := strings.Split(m.Content, " ")
		println(strings.Join(splut, ", "))
		if len(splut) == 1 || !ytsrc.CheckVideoUrl(strings.TrimSpace(splut[1])) {
			_, err := s.ChannelMessageSend(c.ID, "Please provide a correct url")
			if err != nil {
				return
			}
			return
		}

		videoId, err := ytsrc.ExtractVideoId(strings.TrimSpace(splut[1]))

		if err != nil {
			return
		}

		// Find the guild for that channel.
		g, err := s.State.Guild(c.GuildID)
		if err != nil {
			// Could not find guild.
			return
		}

		// Look for the message sender in that guild's current voice states.
		for _, vs := range g.VoiceStates {
			if vs.UserID == m.Author.ID {
				err = playSound(s, g.ID, vs.ChannelID, videoId)
				if err != nil {
					fmt.Println("Error playing sound:", err)
				}

				return
			}
		}
	}
}

// This function will be called (due to AddHandler above) every time a new
// guild is joined.
func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			_, _ = s.ChannelMessageSend(channel.ID, "Airhorn is ready! Type !airhorn while in a voice channel to play a sound.")
			return
		}
	}
}

// loadSound attempts to load an encoded sound file from disk.

// playSound plays the current buffer to the provided channel.
func playSound(s *discordgo.Session, guildID, channelID, videoId string) (err error) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return err
	}

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(250 * time.Millisecond)

	// Start speaking.
	vc.Speaking(true)

	track, err := ytsrc.PlayVideo(videoId)
	if err != nil {
		return err
	}

	url, err := track.Format.GetValidUrl()
	if err != nil {
		return err
	}

	res := seekinghttp.New(url)
	res.Client = &ytsrc.Client

	if size, err := res.Size(); err != nil {
		return err
	} else if size == 0 {
		return errors.New("got an empty request")
	}

	var webm webm2.WebM
	file, err := webm2.Parse(res, &webm)

	if err != nil {
		return err
	}

	for {
		packet, ok := <-file.Chan

		// If this is the end of the file, just return.
		if !ok {
			file.Shutdown()
			break
		}

		// Append encoded pcm data to the buffer.
		vc.OpusSend <- packet.Data
	}

	// Stop speaking
	vc.Speaking(false)

	// Sleep for a specificed amount of time before ending.
	time.Sleep(250 * time.Millisecond)

	// Disconnect from the provided voice channel.
	vc.Disconnect()

	return nil
}
