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

// Package webm provides top level structs that can be used to easily play
// tracks that are encoded in WEBM.
package webm

import (
	"errors"
	"fmt"
	"github.com/dondish/lionplayer/core"
	"github.com/ebml-go/ebml"
	"io"
	"time"
)

/*
Improved version of https://github.com/ebml-go/webm that parses seeks on the Go. has way lower loading times.
*/

// A Top-Level Element of information with many tracks described.
// https://matroska.org/technical/specs/index.html#Tracks
type Tracks struct {
	TrackEntry []TrackEntry `ebml:"AE"`
}

// Describes a track with all Elements.
// https://matroska.org/technical/specs/index.html#TrackEntry
type TrackEntry struct {
	TrackNumber uint64 `ebml:"D7"`
}

// Contains a single seek entry to an EBML Element.
// https://matroska.org/technical/specs/index.html#Seek
type Seek struct {
	SeekId       []byte `ebml:"53AB"`
	SeekPosition uint64 `ebml:"53AC"`
}

// The Parser abstracts the parsing of ebml
type Parser struct {
	*ebml.Element
}

type CuePoint struct {
	timecode  uint64
	positions []uint64
}

// Returns a new parser instance for this input stream
func New(rs io.ReadSeeker) (*Parser, error) {
	var e *ebml.Element
	e, err := ebml.RootElement(rs)
	if err != nil {
		return nil, err
	}
	return &Parser{e}, nil
}

// Parses the SeekHead and returns the position of the Cues element in the segment
func parseMetaSeek(seekhead *ebml.Element) (uint64, error) {
	for seek, err := seekhead.Next(); err == nil; seek, err = seekhead.Next() {
		var rseek Seek
		err = seek.Unmarshal(&rseek)
		if err != nil {
			continue
		}
		if len(rseek.SeekId) > 0 && rseek.SeekId[0] == 0x1C {
			return rseek.SeekPosition, nil
		}
	}
	return 0, errors.New("cues not found")
}

// Parses a Tracks element and returns the track-ids found in each TrackEntry
func parseTracks(tracks *ebml.Element) ([]uint64, error) {
	tracknumbers := make([]uint64, 0)
	var te TrackEntry
	for trackentry, err := tracks.Next(); err == nil; trackentry, err = tracks.Next() {
		err = trackentry.Unmarshal(&te)
		if err != nil {
			return nil, err
		}
		tracknumbers = append(tracknumbers, te.TrackNumber)
	}
	return tracknumbers, nil
}

// Parses a webm file and returns a playable, seek is only supported on non livestream songs.
func (p *Parser) Parse() (core.PlaySeekable, error) {
	ebmlh, err := p.Next()
	if err != nil {
		return nil, err
	}
	if ebmlh.Id != 0x1A45DFA3 {
		return nil, errors.New(fmt.Sprintf("no ebml header provided: %#x", ebmlh.Id))
	}
	_, err = p.Seek(ebmlh.Size(), 1)
	if err != nil {
		return nil, err
	}
	segment, err := p.Next() // Segment
	if err != nil {
		return nil, err
	}
	if segment.Id != 0x18538067 {
		return nil, errors.New(fmt.Sprintf("got something that is not segment: %#x", segment.Id))
	}
	t := Track{
		Output:  make(chan core.Packet),
		seek:    make(chan time.Duration, 3),
		parser:  p,
		segment: segment,
		cues:    0,
		tracks:  make([]uint64, 0),
		trackId: 0,
	}
	for el, err := segment.Next(); err == nil; el, err = segment.Next() {
		switch el.Id {
		case 0x114D9B74: // SeekHead
			pos, err := parseMetaSeek(el)
			if err != nil {
				return nil, err
			}
			if pos > 0 {
				t.cues = int64(pos) + el.Offset
			}
		case 0x1F43B675: // Clusters
			_, err = segment.Seek(el.Offset, 0)
			if err != nil {
				return nil, err
			}
			goto Finish
		case 0x1654AE6B: // Tracks
			tracks, err := parseTracks(el)
			if err != nil {
				return nil, err
			}
			if len(tracks) == 0 {
				return nil, errors.New("no tracks found in segment")
			}
			t.tracks = append(t.tracks, tracks...)
			t.trackId = t.tracks[0]
		case 0x1C53BB6B: // Cues
			t.cuepoints, err = parseCues(el, len(t.tracks))
			if err != nil {
				return nil, err
			}
		}
		_, err = segment.Seek(el.Size(), 1)
	}
Finish:
	return t, nil

}
