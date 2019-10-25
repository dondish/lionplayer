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

package youtube

import (
	"errors"
	"github.com/dondish/lionplayer/core"
	"github.com/dondish/lionplayer/seekablehttp"
	"github.com/dondish/lionplayer/webm"
	"strings"
	"time"
)

// Track represents a Youtube track.
// Track is lazy-loaded which means that it won't
// load anything before being instructed to, which
// means that to extract the valid url you will
// need to call Track.GetPlaySeekable()
type Track struct {
	VideoId  string
	Title    string
	Author   string
	Length   time.Duration
	IsStream bool
	source   *Source
	Format   *Format
}

// Codec returns the codec the content is encoded in.
func (t Track) Codec() string {
	return strings.Trim(strings.Split(t.Format.Type, "=")[1], "\"")
}

// Duration returns the duration of the track.
func (t Track) Duration() time.Duration {
	return t.Length
}

// Playable returns a core.Playable matching this track.
func (t Track) Playable() (core.Playable, error) {
	return t.PlaySeekable()
}

// PlaySeekable returns a core.PlaySeekable matching this track.
func (t Track) PlaySeekable() (core.PlaySeekable, error) {
	vurl, err := t.Format.GetValidUrl()
	if err != nil {
		return nil, err
	}

	res := seekablehttp.New(vurl, t.Format.Clen)

	if size, err := res.Size(); err != nil {
		return nil, err
	} else if size == 0 {
		return nil, errors.New("got an empty request")
	}
	if strings.Split(t.Format.Type, ";")[0] == "audio/webm" {
		parser, err := webm.New(res)

		if err != nil {
			return nil, err
		}

		file, err := parser.Parse()
		if err != nil {
			panic(err)
			return nil, err
		}
		return file, nil
	}
	return nil, errors.New("mime type not supported")
}
