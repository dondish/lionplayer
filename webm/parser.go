package webm

import (
	"errors"
	"fmt"
	"github.com/ebml-go/ebml"
	"io"
	"lionPlayer/core"
	"time"
)

/*
Improved version of https://github.com/ebml-go/webm that parses seeks on the go. has way lower loading times.
*/

const (
	BadTC    = time.Duration(-1000000000000000)
	shutdown = 2 * BadTC
)

type CuePoint struct {
	Time      uint64          `ebml:"B3"`
	Positions []TrackPosition `ebml:"B7"`
}

type TrackPosition struct {
	Track            uint64 `ebml:"F7"`
	ClusterPosition  uint64 `ebml:"F1"`
	RelativePosition uint64 `ebml:"F0"`
}

type Tracks struct {
	TrackEntry []TrackEntry `ebml:"AE"`
}

type TrackEntry struct {
	TrackNumber uint64 `ebml:"D7"`
}

type Seek struct {
	SeekId       []byte `ebml:"53AB"`
	SeekPosition uint64 `ebml:"53AC"`
}

type Parser struct {
	*ebml.Element
}

func NewParser(rs io.ReadSeeker) (*Parser, error) {
	var e *ebml.Element
	e, err := ebml.RootElement(rs)
	if err != nil {
		return nil, err
	}
	return &Parser{e}, nil
}

func (p *Parser) parseMetaSeek(seekhead *ebml.Element) (uint64, error) {
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

/**
Returns a list of track ids in this segment
*/
func (p *Parser) parseTracks(tracks *ebml.Element) ([]uint64, error) {
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

/*
Parse the webm file
*/
func (p *Parser) Parse() (core.PlaySeekable, error) {
	println("p", p.Id)
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
		Output:  make(chan []byte),
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
			cues, err := p.parseMetaSeek(el)
			if err != nil {
				return nil, err
			}
			t.cues = int64(cues)
		case 0x1F43B675: // Clusters
			_, err = segment.Seek(el.Offset, 0)
			t.data = el.Offset
			if err != nil {
				return nil, err
			}
			goto Finish
		case 0x1654AE6B: // Tracks
			tracks, err := p.parseTracks(el)
			if err != nil {
				return nil, err
			}
			if len(tracks) == 0 {
				return nil, errors.New("no tracks found in segment")
			}
			t.tracks = append(t.tracks, tracks...)
			t.trackId = t.tracks[0]
		}
		_, err = p.Seek(el.Size(), 1)
	}
	if err != nil {
		return nil, err
	}
Finish:
	return t, nil

}
