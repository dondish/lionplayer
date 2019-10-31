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

// Package core defines the basic implementation of common data structures used by and exported from lionplayer.
package core

import (
	"io"
	"time"
)

// Packet represents an audio frame (or multiple frames).
type Packet struct {
	// The timecode of this packet.
	Timecode time.Duration
	// The data encoded in the codec of the playable sending this packet.
	Data []byte
}

// Playable is an interface for structs that will be passed to the player.
//
// Playable should not be copied orr passed around but instead should be used by the player itself.
type Playable interface {
	io.Closer
	Chan() <-chan Packet
	Play()
	Pause(bool)
	SampleRate() int
	Channels() int
	Codec() string
}

// PlaySeekable is a Playable where it is possible to seek to a timecode.
type PlaySeekable interface {
	Playable
	Seek(duration time.Duration) error
}

// Track contains metadata that can be used to extract a Playable.
type Track interface {
	// Playable returns a new Playable matching this track.
	Playable() (Playable, error)
	// Bitrate returns the bitrate.
	Bitrate() int
	// Codec returns the codec.
	Codec() string
	// Duration returns the duration of the track.
	Duration() time.Duration
}

// SeekableTrack contains metadata that can be used to extract a PlaySeekable.
type SeekableTrack interface {
	Track
	// PlaySeekable returns a new PlaySeekable matching this track.
	PlaySeekable() (PlaySeekable, error)
}
