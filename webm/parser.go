package webm

import (
	"errors"
	"github.com/ebml-go/ebml"
	"io"
	"lionPlayer/core"
	"sync"
	"time"
)

type Parser struct {
	*ebml.Element
	sync.Mutex
}

func NewParser(rs io.ReadSeeker) (*Parser, error) {
	var e *ebml.Element
	e, err := ebml.RootElement(rs)
	if err != nil {
		return nil, err
	}
	return &Parser{e, sync.Mutex{}}, nil
}

type Track struct {
	Output  <-chan []byte
	close   chan<- struct{}
	parser  *Parser
	segment *ebml.Element
	cues    int64
	trackId uint64
}

func (t Track) Close() error {
	t.close <- struct{}{}
	return nil
}

func (t Track) Chan() <-chan []byte {
	return t.Output
}

type CuePoint struct {
	Time      uint64          `ebml:"B3"`
	Positions []TrackPosition `ebml:"B7"`
}

type TrackPosition struct {
	Track            uint64 `ebml:"F7"`
	ClusterPosition  uint64 `ebml:"F1"`
	RelativePosition uint64 `ebml:"F0"`
}

func (t Track) Seek(duration time.Duration) error {
	_, err := t.segment.Seek(t.cues, 0)
	cues, err := t.segment.Next()
	if err != nil {
		return err
	}
	var lastcuepoint CuePoint
	var cuepoint CuePoint
	for el, err := cues.Next(); err == nil; el, err = cues.Next() {
		lastcuepoint = cuepoint
		err = el.Unmarshal(&cuepoint)
		if err != nil {
			return err
		}
		if time.Duration(cuepoint.Time)*time.Millisecond > duration {
			for _, track := range lastcuepoint.Positions {
				if track.Track == t.trackId {
					_, err := t.segment.Seek(int64(track.ClusterPosition+track.RelativePosition), 0)
					return err
				}
			}
		}
	}
	return errors.New("trackid not found / cue not found")
}

func (t Track) Play() {
	// TODO
}

func (p *Parser) skip(e *ebml.Element, n int) {
	p.Lock()
	i := 0
	for _, err := e.Next(); i < n && err == nil; _, err = e.Next() {
		i++
	}
	p.Unlock()
}

func (p *Parser) Parse() (core.PlaySeekable, error) {
	_, err := p.Next() // Header
	if err != nil {
		return nil, err
	}
	segment, err := p.Next() // Segment
	if err != nil {
		return nil, err
	}
	p.skip(segment, 4) // Skip over unnecessary information
	clusters, err := segment.Next()
	if err != nil {
		return nil, err
	}
	cueingpoint, err := segment.Next()
	if err != nil {
		return nil, err
	}
	cues, err := cueingpoint.Next()
	if err != nil {
		return nil, err
	}
	c := make(chan []byte)
	close := make(chan struct{})
	return &Track{
		Output: c,
		parser: p,
		close:  close,
	}, nil

}
