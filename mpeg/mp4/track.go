package mp4

import (
	"github.com/dondish/lionplayer/core"
)

type Track struct {
	Tracks   []TrackEntry
	Root     *Element
	Metadata map[string]interface{}
}

func (t Track) Close() error {
	panic("implement me")
}

func (t Track) Chan() <-chan core.Packet {
	panic("implement me")
}

func (t Track) Play() {
	panic("implement me")
}

func (t Track) Pause(bool) {
	panic("implement me")
}

type StandardTrack struct {
	Track
}

type FragmentedTrack struct {
	Track
}
