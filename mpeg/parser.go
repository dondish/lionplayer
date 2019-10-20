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
	"errors"
	"io"
	"lionplayer/core"
)

type TrackEntry struct {
	Id      uint32
	Handler string
}

type Parser struct {
	*Element
	isFragmented bool
	sampleTables []*Element
}

// Creates a new Parser
func New(rs io.ReadSeeker, length int64) *Parser {
	return &Parser{Element: &Element{
		R:      rs,
		N:      length,
		Offset: 0,
		Id:     "root",
		intbuf: make([]byte, 4),
	}, sampleTables: make([]*Element, 0)}
}

func (p *Parser) handleMdia(track *Track, trak *Element) error {
	var el *Element
	var err error
	te := TrackEntry{
		Id:      0,
		Handler: "",
	}
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
	return err
}

//func (p *Parser) handleMinf(minf *Element) error {
//
//}

func (p *Parser) handleTrak(te *TrackEntry, trak *Element) error {
	var el *Element
	var err error
	for el, err = trak.Next(); err == nil; el, err = trak.Next() {
		switch el.Id {
		case "hdlr":
			err = el.ParseFlags()
			if err != nil {
				return err
			}
			_, err = el.R.Seek(4, 1)
			if err != nil {
				return err
			}
			te.Handler, err = el.readType()
			if err != nil {
				return err
			}
		}
		err = el.Skip()
	}
	return err
}

func (p *Parser) handleMoov(track *Track, moov *Element) error {
	var el *Element
	var err error
	te := &TrackEntry{
		Id:      0,
		Handler: "",
	}
	for el, err = moov.Next(); err == nil; el, err = moov.Next() {
		switch el.Id {
		case "trak":
			err = p.handleTrak(te, el)
			if err != nil {
				return err
			}
		}
		err = el.Skip()
	}
	return err
}

// Returns a playable, will also implement playseekable if possible
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
	track := &Track{
		Tracks:   make([]TrackEntry, 0),
		Root:     p.Element,
		Metadata: make(map[string]interface{}),
	}
	for el, err := p.Next(); err == nil; el, err = p.Next() {
		switch el.Id {
		case "mdat", "free":
			_, err = p.R.Seek(el.Offset, 0)
			goto Finish
		case "moov":
			err = p.handleMoov(track, el)
			if err != nil {
				return nil, err
			}
		}
		err = el.Skip()
	}
Finish:
	return track, nil
}
