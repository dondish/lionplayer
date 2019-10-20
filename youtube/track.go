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
	"lionplayer/core"
	"lionplayer/seekablehttp"
	"lionplayer/webm"
	"strings"
	"time"
)

type Track struct {
	VideoId  string
	Title    string
	Author   string
	Duration time.Duration
	IsStream bool
	source   *Source
	Format   *Format
}

func (t Track) GetBitrate() int {
	return int(t.Format.Bitrate)
}

func (t Track) GetChannels() int {
	return 2
}

func (t Track) GetCodec() string {
	return strings.Trim(strings.Split(t.Format.Type, "=")[1], "\"")
}

func (t Track) GetDuration() time.Duration {
	return t.Duration
}

// Return a playable of this track that can be played.
func (t Track) GetPlayable() (core.Playable, error) {
	return t.GetPlaySeekable()
}

// Return a playseekable of this track that can be played.
func (t Track) GetPlaySeekable() (core.PlaySeekable, error) {
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
	} else {
		return nil, errors.New("mime type not supported")
	}
}
