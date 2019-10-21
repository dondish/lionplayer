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
	"log"
	"time"
)

const (
	badTC    = time.Duration(-1000000000000000) // No signal
	shutdown = 2 * badTC                        // Shutdown signal
	pause    = 3 * badTC                        // Pause signal
	unpause  = 4 * badTC                        // Unpause signal
)

// Basic container of information containing a single Block and information specific to that Block.
// https://matroska.org/technical/specs/index.html#BlockGroup
type BlockGroup struct {
	Block []byte `ebml:"A1"` // Block containing the actual data to be rendered and a timestamp relative to the Cluster Timestamp.
}

// The Top-Level Element containing the (monolithic) Block structure.
// https://matroska.org/technical/specs/index.html#Cluster
type Cluster struct {
	SimpleBlock []byte `ebml:"A3" ebmlstop:"1"` // Similar to Block but without all the extra information,
	// mostly used to reduced overhead when no extra feature is needed.
	Timecode   uint       `ebml:"E7"` // Absolute timestamp of the cluster.
	PrevSize   uint       `ebml:"AB"` // Size of the previous Cluster, in octets. Can be useful for backward playing.
	Position   uint       `ebml:"A7"` // The Segment Position of the Cluster in the Segment (0 in live streams). It might help to resynchronise offset on damaged streams.
	BlockGroup BlockGroup `ebml:"A0" ebmlstop:"1"`
}

// The track is returned by the Parser
// The track implements the core.Playable and core.PlaySeekable interface,
// Although seeking in a live-stream will return an error.
type Track struct {
	Output     chan core.Packet   // Output channel of Packet instances
	seek       chan time.Duration // signal channel
	parser     *Parser            // The parser responsible for this Track
	segment    *ebml.Element      // The segment
	cues       int64              // the position of the cues element
	cuepoints  []CuePoint         // saved cuepoints
	tracks     []TrackEntry       // all of the tracks' ids
	trackId    uint64             // the current track id
	samplerate int                // the sample rate
	channels   int                // the channel count
	codec      string             // the codec the playable returns
}

func (t Track) SampleRate() int {
	return t.samplerate
}

func (t Track) Channels() int {
	return t.channels
}

func (t Track) Codec() string {
	return t.codec
}

// Contain positions for different tracks corresponding to the timestamp.
// https://matroska.org/technical/specs/index.html#TrackPosition
type TrackPosition struct {
	Track            uint64 `ebml:"F7"`
	ClusterPosition  uint64 `ebml:"F1"`
	RelativePosition uint64 `ebml:"F0"`
}

// Sets the pause to the boolean given
// For example: if set to true, the player will pause
// While pausing, until resumed you can't seek.
func (t Track) Pause(b bool) {
	if b {
		t.seek <- pause // send the pause signal
	} else {
		t.seek <- unpause // send the resume signal
	}
}

// Stops the given track
// Resources should be picked up by the GC and cleaned
func (t Track) Close() error {
	t.segment = nil
	t.parser = nil
	t.seek <- shutdown
	return nil
}

// Returns the output channel
func (t Track) Chan() <-chan core.Packet {
	return t.Output
}

func readUint64(e *ebml.Element) (uint64, error) {
	d, err := e.ReadData()
	var i int
	sz := len(d)
	var val uint64
	for i = 0; i < sz; i++ {
		val <<= 8
		val += uint64(d[i])
	}
	return val, err
}

// get the cluster positions from the cuepoint postions element
func getClusterPositions(positions *ebml.Element, tracklen int) (poses []uint64, err error) {
	poses = make([]uint64, tracklen+1)
	var pos *ebml.Element
	var trac uint64
	var position uint64
	var clusterpos *ebml.Element
	for pos, err = positions.Next(); err == nil && pos.Id == 0xF7; pos, err = positions.Next() {
		trac, err = readUint64(pos)

		clusterpos, err = positions.Next()
		if err != nil {
			return
		}
		position, err = readUint64(clusterpos)
		poses[trac] = position
		_, err = positions.Seek(pos.Size(), 1)
	}
	if err == io.EOF {
		err = nil
	}
	return
}

// get the cuepoints from the cues element
func parseCues(cues *ebml.Element, tracklen int) ([]CuePoint, error) {
	if cues.Id != 0x1C53BB6B {
		log.Println("wrong cues id", fmt.Sprintf("%#x", cues.Id))
	}
	cuepoints := make([]CuePoint, 0)
	var tim *ebml.Element
	var timecode uint64
	for el, err := cues.Next(); err == nil && el.Id == 0xBB; el, err = cues.Next() { // Go over the cuepoints
		tim, err = el.Next()
		if err != nil {
			return nil, err
		}
		timecode, err = readUint64(tim)
		if err != nil {
			return nil, err
		}
		positions, err := el.Next()
		if err != nil {
			return nil, err
		}
		poses, err := getClusterPositions(positions, tracklen)
		cuepoints = append(cuepoints, CuePoint{
			timecode:  timecode,
			positions: poses,
		})
		_, err = cues.Seek(el.Size(), 1)
	}
	return cuepoints, nil
}

// Seeks to the cluster just before the timecode given
func (t Track) internalSeek(duration time.Duration) error {
	if t.cues == 0 {
		return errors.New("seeks are not supported in streams")
	}
	if t.cuepoints == nil {
		_, err := t.segment.Seek(t.cues, 0)
		if err != nil {
			return err
		}
		cues, err := t.segment.Next()
		t.cuepoints, err = parseCues(cues, len(t.tracks))
		if err != nil {
			return err
		}
	}
	var lastpos uint64 = 0
	for _, cuepoint := range t.cuepoints {
		if time.Duration(cuepoint.timecode)*time.Millisecond > duration {
			_, err := t.segment.Seek(t.segment.Offset+12+int64(lastpos), 0)
			return err
		}
		lastpos = cuepoint.positions[t.trackId]
	}
	_, err := t.segment.Seek(t.segment.Offset+12+int64(lastpos), 0)
	return err
}

// Sends a seek signal to the player, it will seek to that position after finishing up with the current cluster.
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

func (t Track) sendLaces(d []byte, sz []int, pos time.Duration) {
	var curr int
	final := make([]byte, len(d))
	for i, l := 0, len(sz); i < l; i++ {
		if sz[i] != 0 {
			final = d[curr : curr+sz[i]]
			t.Output <- core.Packet{
				Timecode: pos,
				Data:     final,
			}
			curr += sz[i]
		}
	}
	t.Output <- core.Packet{
		Timecode: pos,
		Data:     final,
	}
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

func (t *Track) handleBlock(block []byte, currtime time.Duration) {
	pos := currtime + time.Duration(uint(block[1])<<8+uint(block[2]))*time.Millisecond
	lacing := (block[3] >> 1) & 3
	switch lacing {
	case 0:
		t.Output <- core.Packet{
			Timecode: pos,
			Data:     block[4:],
		}
	case 1:
		sz, curr := parseXiphSizes(block)
		t.sendLaces(block[curr:], sz, pos)
	case 2:
		sz, curr := parseFixedSizes(block)
		t.sendLaces(block[curr:], sz, pos)
	case 3:
		sz, curr := parseEBMLSizes(block)
		t.sendLaces(block[curr:], sz, pos)

	}
}

func (t *Track) handleCluster(cluster *ebml.Element, currtime time.Duration) {
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
				t.handleBlock(block, currtime)
			}
		}
	}
}

// Starts parsing the content of the track and populating the output channel.
// The output channel will be automatically closed after this.
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
		if err != nil && err.Error() == "Reached payload" { // Found a block of data
			t.handleCluster(err.(ebml.ReachedPayloadError).Element, time.Millisecond*time.Duration(c.Timecode))
			err = nil
		}
		seek := badTC
		for len(t.seek) != 0 {
			seek = <-t.seek
		}
		if seek == pause {
			for seek != unpause {
				seek = <-t.seek
				if seek == shutdown {
					break
				}
			}
		}
		if seek == shutdown {
			break
		}
		if seek != badTC {
			err = t.internalSeek(seek)
		}
	}
	if err != nil && err != io.EOF {
		log.Println("play error", err)
	}
}
