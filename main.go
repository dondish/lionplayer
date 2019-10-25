/*
MIT License

Copyright (c) 2019 Oded Shapira

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package main is responsible for launching the bot.
package main

import (
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dondish/lionplayer/core"
	"github.com/dondish/lionplayer/youtube"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	seekPattern, _ = regexp.Compile("(?:([0-9]{1,2})h)?(?:([0-9]{1,2})m)?(?:([0-9]{1,2})s)?") // A pattern to seek
)

func init() {
	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

var token string

var ytsrc = youtube.New(nil)
var tracks = make(map[string]core.Playable)
var lastpacket core.Packet

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
	fmt.Println("Lionplayer is now running.  Press CTRL-C to exit.")
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
	s.UpdateStatus(0, "Playing music using Go only!")
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
	track, ok := tracks[m.GuildID]
	if strings.HasPrefix(m.Content, "!!play") && !ok {
		// Find the channel that the message came from.
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			// Could not find channel.
			return
		}

		splut := strings.Split(m.Content, " ")

		if len(splut) == 1 || !ytsrc.CheckVideoUrl(strings.TrimSpace(splut[1])) {
			_, err := s.ChannelMessageSend(c.ID, "Please provide a correct url")
			if err != nil {
				return
			}
			return
		}

		videoID, err := ytsrc.ExtractVideoId(strings.TrimSpace(splut[1]))

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
				err = playSound(s, g.ID, vs.ChannelID, videoID, c.ID)
				if err != nil {
					fmt.Println("Error playing sound:", err)
				}

				return
			}
		}
	} else if strings.HasPrefix(m.Content, "!!stop") {
		if track != nil {
			track.Close()
		}
	} else if strings.HasPrefix(m.Content, "!!seek") {
		// Find the channel that the message came from.
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			// Could not find channel.
			return
		}

		if track == nil {
			_, err := s.ChannelMessageSend(c.ID, "Not playing anything")
			if err != nil {
				return
			}
			return

		}

		splut := strings.Split(m.Content, " ")

		if len(splut) == 1 || !seekPattern.MatchString(strings.TrimSpace(splut[1])) {
			_, err := s.ChannelMessageSend(c.ID, "Please provide a correct seek")
			if err != nil {
				return
			}
			return
		}
		matches := seekPattern.FindStringSubmatch(strings.TrimSpace(splut[1]))
		var hour, minute, second int
		if matches[1] != "" {
			hour, _ = strconv.Atoi(matches[1])
		}
		if matches[2] != "" {
			minute, _ = strconv.Atoi(matches[2])
		}
		if matches[3] != "" {
			second, _ = strconv.Atoi(matches[3])
		}
		ms := time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second
		trackseek, ok := track.(core.PlaySeekable)
		if ok {
			_ = trackseek.Seek(ms)
		} else {
			_, _ = s.ChannelMessageSend(c.ID, "Track is not seekable")
			return
		}
	} else if strings.HasPrefix(m.Content, "!!pause") {
		if track != nil {
			track.Pause(true)
		}
	} else if strings.HasPrefix(m.Content, "!!unpause") || strings.HasPrefix(m.Content, "!resume") {
		if track != nil {
			track.Pause(false)
		}
	} else if strings.HasPrefix(m.Content, "!!position") {
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			// Could not find channel.
			return
		}

		if track != nil {
			_, err = s.ChannelMessageSend(c.ID, lastpacket.Timecode.String())
			if err != nil {
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
func playSound(s *discordgo.Session, guildID, channelID, videoID, msgchannel string) (err error) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return err
	}

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(250 * time.Millisecond)

	// Start speaking.
	vc.Speaking(true)

	defer func() {
		// Stop speaking
		vc.Speaking(false)

		// Sleep for a specificed amount of time before ending.
		time.Sleep(250 * time.Millisecond)

		// Disconnect from the provided voice channel.
		vc.Disconnect()
	}()

	trac, err := ytsrc.PlayVideo(videoID)
	if err != nil {
		return err
	}

	if trac.IsStream {
		_, err = s.ChannelMessageSend(msgchannel, fmt.Sprintf("Now Playing - %s - %s [LIVE]", trac.Title, trac.Author))
	} else {
		_, err = s.ChannelMessageSend(msgchannel, fmt.Sprintf("Now Playing - %s - %s [%s]", trac.Title, trac.Author, trac.Length))
	}
	if err != nil {
		return err
	}
	file, err := trac.PlaySeekable()

	if err != nil {
		return err
	}
	tracks[guildID] = file
	go file.Play()
	c := file.Chan()
	for {
		packet, ok := <-c
		lastpacket = packet
		// If this is the end of the file, just return.
		if !ok {
			_ = file.Close()
			break
		}

		// Append encoded pcm data to the buffer.
		vc.OpusSend <- packet.Data
	}

	delete(tracks, guildID)

	return nil
}
