package core

import (
	"io"
	"time"
)

type Playable interface {
	io.Closer
	Chan() <-chan []byte // returns the output channel
	Play()               // Start this function in a new coroutine, it feeds the channel data
}

type PlaySeekable interface {
	Playable
	Seek(duration time.Duration) error // Seek to a point in the track
}
