package core

import (
	"errors"
	"github.com/gorilla/websocket"
	"math/rand"
	"sync"
	"time"
)

type VoiceState struct {
	GuildId string `json:"guild_id"`
	ChannelId string `json:"channel_id"`
	UserId string `json:"user_id"`
	Deaf bool `json:"deaf"`
	Mute bool `json:"mute"`
	// All other properties are irrelevant
}

type ReadyPayload struct {
	Ssrc int `json:"ssrc"`
	Ip string `json:"ip"`
	Port int `json:"port"`
	Modes []string `json:"modes"`
	// All other properties are irrelevant
}

type GuildVoiceStatus struct {
	Token string // Voice Connection Token
	GuildId string // The guild's id
	SessionId string // the session id
	Endpoint string // The voice server host
	State VoiceState // The current voice state
	Conn *websocket.Conn // The connection
	Ssrc int // The ssrc
	Ip string // The UDP server's ip
	Port int // The UDP server's port
	Mode string // The chosen mode
	stopel chan bool // Stop the event listener
	stophb chan bool // Stop the heartbeat
}

type Payload struct {
	Op int `json:"op"` // The opcode for the payload
	D interface{} `json:"d"` // Event data
	S *int `json:"s,omitempty"` // Sequence Number
	T string `json:"t,omitempty"` // Event name
}

func (status *GuildVoiceStatus) startEventListener(c chan <- interface{}) {
	for {
		select {
		 case <- status.stopel:
		 	return
		default:
			var payload Payload
			err := status.Conn.ReadJSON(payload)
			if err != nil {
				panic("failed to read from the websocket")
			}
			if payload.Op == 8 {
				if status.stophb != nil { // Stop the previous heartbeat routine if exists
					status.stophb <- true
				}
				status.stophb = make(chan bool)
				go status.startHeartBeatRoutine(time.Millisecond * payload.D.(map[string]interface{})["heartbeat_interval"].(time.Duration))
				c <- payload
			}
		}
	}
}

func (status *GuildVoiceStatus) startHeartBeatRoutine(ml time.Duration) {
	for {
		select {
			case <- status.stophb:
				return
			case <- time.After(ml):
				err := status.Conn.WriteJSON(Payload{
					Op: 3,
					D:  rand.Int(),

				})
				if err != nil {
					panic("failed to send a heartbeat")
				}
		}
	}
}

func (status *GuildVoiceStatus) Close() {
	status.stopel <- true
	status.stophb <- true
}

func getBestMode(modes []string) string {
	if len(modes) == 0 {
		return "plain"
	}
	for _, v := range modes {
		if v == "xsalsa20_poly1305" {
			return v
		}
	}
	return modes[0]
}

// The user will establish the voice connection himself and only forward the endpoint so that lionPlayer can connect to the endpoint and handle audio.
func (status *GuildVoiceStatus) ConnectToVoice() (*EventListener, error) {
	// First off, connect to the voice WS
	conn, res, err := websocket.DefaultDialer.Dial(status.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, errors.New("status code is not 200")
	}
	status.Conn = conn
	c := make(chan interface{})
	status.stopel = make(chan bool)
	go status.startEventListener(c)
	identify := Payload{
		Op: 0,
		D: map[string]interface{} {
			"server_id": status.GuildId,
			"user_id": status.State.UserId,
			"session_id": status.SessionId,
			"token": status.Token,
		},
	}
	err = conn.WriteJSON(identify)
	if err != nil {
		return nil, err
	}
	// The first payload received should be the ready payload that includes the information we need to connect to the UDP servers
	ready := (<- c).(Payload)
	if ready.Op != 2 {
		return nil, errors.New("should send a ready payload first")
	}
	readypl := ready.D.(ReadyPayload)
	status.Ip = readypl.Ip
	status.Port = readypl.Port
	status.Mode = getBestMode(readypl.Modes)
	status.Ssrc = readypl.Ssrc

	listener := EventListener{
		Channel:    c,
		Listeners:  make([]Listener, 1),
		WaitingFor: sync.Map{},
	}
	go listener.StartListener()

	// TODO add UDP Connection
	return &listener, nil
}