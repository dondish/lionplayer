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

// Packet represents an audio frame (or multiple frames)
type Packet struct {
	// The timecode of this packet
	Timecode time.Duration
	// The data encoded in the codec of the playable sending this packet
	Data []byte
}

// Playable defines methods that should be implemented
// Playable should not be copied orr passed around but instead should be used by the player itself
type Playable interface {
	io.Closer            // Will be used to stop the track
	Chan() <-chan Packet // Returns the output channel
	Play()               // Start this function in a new coroutine, it feeds the channel data
	Pause(bool)          // Whether to pause or unpause
	SampleRate() int     // Returns the sampling frequency in Hz
	Channels() int       // Returns the amount of channels
	Codec() string       // Returns the codec (lowercase) (for example: opus)
}

// The interface of a playable that can be seek in
type PlaySeekable interface {
	Playable
	Seek(duration time.Duration) error // Seek to a point in the track
}

// An interface for a track
// A track is not played directly and can be passed by multiple nodes
type Track interface {
	Playable() (Playable, error) // Get a playable matching this track. Can be called multiple times to extract a new Playable each time.
	Bitrate() int                // Returns the bitrate the track will use
	Codec() string               // Returns the codec the playable is encoded in
	Duration() time.Duration     // Returns the duration of the track
}

// An interface for a track that can be seeked
type SeekableTrack interface {
	Track
	PlaySeekable() (PlaySeekable, error)
}
