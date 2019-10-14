package webm

import (
	"fmt"
	"github.com/ebml-go/ebml"
	"io"
	"log"
	"time"
)

type Track struct {
	Output  chan []byte
	seek    chan time.Duration
	parser  *Parser
	segment *ebml.Element
	data    int64
	cues    int64
	tracks  []uint64
	trackId uint64
}

type BlockGroup struct {
	Block []byte `ebml:"A1"`
}

type Cluster struct {
	SimpleBlock []byte     `ebml:"A3" ebmlstop:"1"`
	Timecode    uint       `ebml:"E7"`
	PrevSize    uint       `ebml:"AB"`
	Position    uint       `ebml:"A7"`
	BlockGroup  BlockGroup `ebml:"A0" ebmlstop:"1"`
}

func (t Track) Close() error {
	t.seek <- shutdown
	return nil
}

func (t Track) Chan() <-chan []byte {
	return t.Output
}

func (t Track) internalSeek(duration time.Duration) error {
	curr, err := t.segment.Seek(0, 1)
	_, err = t.segment.Seek(t.cues, 0)
	cues, err := t.segment.Next()
	if err != nil {
		return err
	}
	if cues.Id != 0x1C53BB6B {
		log.Println("wrong cues id", fmt.Sprintf("%#x", cues.Id))
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
					log.Println("found it")
					_, err := t.segment.Seek(t.data+int64(track.ClusterPosition), 0)
					return err
				}
			}
		}
	}
	_, err = t.segment.Seek(curr, 0) // if cannot seek, seek back to where it was
	return err
}

func (t Track) Seek(duration time.Duration) error {
	t.seek <- duration
	return nil
}

func remaining(x int8) (rem int) {
	for x > 0 {
		rem++
		x += x
	}
	return
}

func laceSize(v []byte) (val int, rem int) {
	val = int(v[0])
	rem = remaining(int8(val))
	for i, l := 1, rem+1; i < l; i++ {
		val <<= 8
		val += int(v[i])
	}
	val &= ^(128 << uint(rem*8-rem))
	return
}

func laceDelta(v []byte) (val int, rem int) {
	val, rem = laceSize(v)
	val -= (1 << (uint(7*(rem+1) - 1))) - 1
	return
}

func (t Track) sendLaces(d []byte, sz []int) {
	var curr int
	final := make([]byte, len(d))
	for i, l := 0, len(sz); i < l; i++ {
		if sz[i] != 0 {
			final = d[curr : curr+sz[i]]
			t.Output <- final
			curr += sz[i]
		}
	}
	t.Output <- d[curr:]
}

func parseXiphSizes(d []byte) (sz []int, curr int) {
	laces := int(uint(d[4]))
	sz = make([]int, laces)
	curr = 5
	for i := 0; i < laces; i++ {
		for d[curr] == 255 {
			sz[i] += 255
			curr++
		}
		sz[i] += int(uint(d[curr]))
		curr++
	}
	return
}

func parseFixedSizes(d []byte) (sz []int, curr int) {
	laces := int(uint(d[4]))
	curr = 5
	fsz := len(d[curr:]) / (laces + 1)
	sz = make([]int, laces)
	for i := 0; i < laces; i++ {
		sz[i] = fsz
	}
	return
}

func parseEBMLSizes(d []byte) (sz []int, curr int) {
	laces := int(uint(d[4]))
	sz = make([]int, laces)
	curr = 5
	var rem int
	sz[0], rem = laceSize(d[curr:])
	for i := 1; i < laces; i++ {
		curr += rem + 1
		var dsz int
		dsz, rem = laceDelta(d[curr:])
		sz[i] = sz[i-1] + dsz
	}
	curr += rem + 1
	return
}

func (t Track) handleBlock(block []byte) {
	lacing := (block[3] >> 1) & 3
	switch lacing {
	case 0:
		t.Output <- block[4:]
	case 1:
		sz, curr := parseXiphSizes(block)
		t.sendLaces(block[curr:], sz)
	case 2:
		sz, curr := parseFixedSizes(block)
		t.sendLaces(block[curr:], sz)
	case 3:
		sz, curr := parseEBMLSizes(block)
		t.sendLaces(block[curr:], sz)

	}
}

func (t Track) handleCluster(cluster *ebml.Element) {
	var err error
	for err == nil && len(t.seek) == 0 {
		var e *ebml.Element
		e, err = cluster.Next()
		var block []byte
		if err == nil {
			switch e.Id {
			case 0xa3: // Block
				block, _ = e.ReadData()
			case 0xa0: // BlockGroup
				var bg BlockGroup
				err = e.Unmarshal(&bg)
				if err == nil {
					block = bg.Block
				}
			}
			if err == nil && block != nil && len(block) > 4 {
				t.handleBlock(block)
			}
		}
	}
}

func (t Track) Play() {
	var err error
	defer close(t.Output)
	for err == nil {
		var c Cluster
		var data *ebml.Element
		data, err = t.segment.Next()
		if err == nil {
			err = data.Unmarshal(&c)
		}
		if err != nil && err.Error() == "Reached payload" {
			t.handleCluster(err.(ebml.ReachedPayloadError).Element)
			err = nil
		}
		seek := BadTC
		for len(t.seek) != 0 {
			seek = <-t.seek
		}
		if err == io.EOF {
			log.Println("EOF")
			seek = <-t.seek
			if seek != BadTC {
				err = nil
			}
		}
		if seek == shutdown {
			log.Println("shutting down")
			break
		}
		if seek != BadTC {
			log.Println("seek", seek)
			err = t.internalSeek(seek)
		}
	}
	log.Println("play error", err)
}
