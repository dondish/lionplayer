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

package mpeg

import (
	"github.com/dondish/lionplayer/core"
)

// Track is a Playable that is MP4 encoded (and PlaySeekable if possible).
type Track struct {
	Tracks   []TrackEntry
	Root     *Element
	Metadata map[string]interface{}
}

// SampleRate returns the samplerate of the Track.
func (t Track) SampleRate() int {
	panic("implement me")
}

// Channels returns the amount of channels of the track.
//
// The result can be 1 (mono) or 2 (stereo).
func (t Track) Channels() int {
	panic("implement me")
}

// Codec returns the codec each Packet returned by the Track is encoded in.
func (t Track) Codec() string {
	panic("implement me")
}

// Close stops the Track and frees up resources.
func (t Track) Close() error {
	panic("implement me")
}

// Chan returns the channel the Track outputs the Packets into.
func (t Track) Chan() <-chan core.Packet {
	panic("implement me")
}

// Play starts parsing the Track populating the channel returned by Chan, closing it on finishing.
//
// Be warned that Play does block the current goroutine, this function should
// be started in a new goroutine.
func (t Track) Play() {
	panic("implement me")
}

// Pause pauses or unpauses the track according to the boolean given.
func (t Track) Pause(bool) {
	panic("implement me")
}
