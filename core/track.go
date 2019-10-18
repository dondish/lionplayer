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

package core

import (
	"io"
	"time"
)

// Decoded packet
type Packet struct {
	Timecode time.Duration
	Data     []byte
}

// The basic interface of a playable interface
type Playable interface {
	io.Closer            // Will be used to stop the track
	Chan() <-chan Packet // returns the output channel
	Play()               // Start this function in a new coroutine, it feeds the channel data
	Pause(bool)          // Whether to pause or unpause
}

// The interface of a playable that can be seek in
type PlaySeekable interface {
	Playable
	Seek(duration time.Duration) error // Seek to a point in the track
}

// An interface for a track
type Track interface {
	GetPlayable() (Playable, error)
	GetBitrate() int
	GetChannels() int
	GetCodec() string
}

// An interface for a track that can be seeked
type SeekableTrack interface {
	Track
	GetPlaySeekable() (PlaySeekable, error)
}
