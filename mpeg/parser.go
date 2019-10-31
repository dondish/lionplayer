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

// Package mpeg simplifies the parsing of the MP4 and other related MPEG file formats.
package mpeg

import (
	"errors"
	"github.com/dondish/lionplayer/core"
	"io"
)

/*
TrackEntry represents an MPEG track.

There can be multiple in each track.
*/
type TrackEntry struct {
	Id         uint32
	Handler    string
	Channels   uint16
	SampleRate uint32
}

// Parser reads and decodes data from an input stream to create a playable instance.
type Parser struct {
	*Element
	// Whether the track is fragmented.
	isFragmented bool
	// The MPEG track types.
	ft *FragmentedTrack
	st *StandardTrack
	// On fragmented MP4 free might be sooner than moov.
	moovReached bool
	// The sample tables to provide seeking.
	sampleTables []*Element
}

// New creates a new Parser using the given ReadSeeker.
func New(rs io.ReadSeeker, length int64) *Parser {
	t := &Track{}
	return &Parser{Element: &Element{
		R:      rs,
		N:      length,
		Offset: 0,
		Id:     "root",
		intbuf: make([]byte, 4),
	},
		sampleTables: make([]*Element, 0),
		ft:           &FragmentedTrack{Track: t},
		st:           &StandardTrack{Track: t},
	}
}

// handleSoun handles an AudioSampleEntry element.
func (p *Parser) handleSoun(te *TrackEntry, soun *Element) error {
	err := soun.ParseFlags()
	if err != nil {
		return err
	}
	_, err = soun.R.Seek(12, 1)
	if err != nil {
		return err
	}
	te.Channels, err = soun.readInt16()
	if err != nil {
		return err
	}
	_, err = soun.R.Seek(4, 1)
	if err != nil {
		return err
	}
	te.SampleRate, err = soun.readInt32()
	return err
}

// handleStsd handles an Sample Description Box element.
func (p *Parser) handleStsd(te *TrackEntry, stsd *Element) error {
	err := stsd.ParseFlags()
	if err != nil {
		return err
	}
	entrycount, err := stsd.readInt32()
	if err != nil {
		return err
	}
	if entrycount > 0 {
		codec, err := stsd.Next()
		if err != nil {
			return err
		}
		if te.Handler == "soun" {
			err = p.handleSoun(te, codec)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// handleStbl handles a Sample Table Box element.
func (p *Parser) handleStbl(te *TrackEntry, stbl *Element) error {
	var el *Element
	var err error
	for el, err = stbl.Next(); err == nil; el, err = stbl.Next() {
		switch el.Id {
		case "stsd":
			err = p.handleStsd(te, el)
			if err != nil {
				return err
			}
		}
		err = el.Skip()
	}
	return err
}

// handleMinf handles a Media Information Box element.
func (p *Parser) handleMinf(te *TrackEntry, minf *Element) error {
	var el *Element
	var err error
	for el, err = minf.Next(); err == nil; el, err = minf.Next() {
		switch el.Id {
		case "stbl":
			err = p.handleStbl(te, minf)
			if err != nil {
				return err
			}
		}
		err = el.Skip()
	}
	return err
}

// handleMdhd handles a Media Header Box element.
func (p *Parser) handleMdhd(te *TrackEntry, mdhd *Element) error {
	err := mdhd.ParseFlags()
	if err != nil {
		return err
	}
	if mdhd.Version == 1 {
		_, err = mdhd.R.Seek(16, 1) // Seek over creation time and modification time
	} else {
		_, err = mdhd.R.Seek(8, 1) // Seek over creation time and modification time
	}
	if err != nil {
		return err
	}
	timescale, err := mdhd.readInt32()
	if err != nil {
		return err
	}
	p.st.TimeScales[int(te.Id)] = int(timescale)
	return nil
}

// handleMdia handles a Media Box element.
func (p *Parser) handleMdia(te *TrackEntry, mdia *Element) error {
	var el *Element
	var err error
	for el, err = mdia.Next(); err == nil; el, err = mdia.Next() {
		switch el.Id {
		case "hdlr": // Handler Reference Box
			err = el.ParseFlags()
			if err != nil {
				return err
			}
			_, err = el.R.Seek(4, 1) // skip over the pre_defined property
			if err != nil {
				return err
			}
			te.Handler, err = el.readType() // should be soun for sounds
			if err != nil {
				return err
			}
		case "mdhd": // Media Header Box
			err = p.handleMdhd(te, el)
		}
		err = el.Skip()
	}
	return err
}

// handleTrak handles a Track Box element.
func (p *Parser) handleTrak(te *TrackEntry, trak *Element) error {
	var el *Element
	var err error
	for el, err = trak.Next(); err == nil; el, err = trak.Next() {
		switch el.Id {
		case "tkhd":
			err = el.ParseFlags()
			if err != nil {
				return err
			}
			if el.Version == 1 {
				_, err = el.R.Seek(16, 1)
			} else {
				_, err = el.R.Seek(8, 1)
			}
			if err != nil {
				return err
			}
			te.Id, err = el.readInt32()
			if err != nil {
				return err
			}
		}
		err = el.Skip()
	}
	return nil
}

// handleMoov handles a Movie Box element.
func (p *Parser) handleMoov(track *Track, moov *Element) error {
	var el *Element
	var err error
	track.Tracks = make([]TrackEntry, 0)
	for el, err = moov.Next(); err == nil; el, err = moov.Next() {
		switch el.Id {
		case "trak":
			te := &TrackEntry{
				Id:      0,
				Handler: "",
			}
			err = p.handleTrak(te, el)
			if err != nil {
				return err
			}
			if te.Handler == "soun" {
				track.Tracks = append(track.Tracks, *te)
			}
		}
		err = el.Skip()
	}
	return nil
}

// Parse parses the headers and returns a playable.
// The playable will also implement PlaySeekable if seeking is possible.
// The playable can be either FragmentedTrack or StandardTrack
func (p *Parser) Parse() (core.Playable, error) {
	ftyp, err := p.Next()
	if err != nil {
		return nil, err
	}
	if ftyp.Id != "ftyp" {
		return nil, errors.New("file type missing")
	}
	err = ftyp.Skip()
	if err != nil {
		return nil, err
	}
	for el, err := p.Next(); err == nil; el, err = p.Next() {
		switch el.Id {
		case "mdat", "free":
			if p.moovReached {
				_, err = p.R.Seek(el.Offset, 0)
				if err != nil {
					return nil, err
				}
				goto Finish
			}
		case "moov":
			p.moovReached = true
			err = p.handleMoov(p.st.Track, el)
			if err != nil {
				return nil, err
			}
		}
		err = el.Skip()
	}
Finish:
	if p.isFragmented {
		return p.ft, nil
	}
	return p.st, nil
}
