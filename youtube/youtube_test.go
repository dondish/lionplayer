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
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	rickvid   = "dQw4w9WgXcQ"
	rick1     = "https://youtu.be/dQw4w9WgXcQ"
	rick2     = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	rick3     = "https://youtube.c/dQw4w9gXcQ"
	ricktitle = "Rick Astley - Never Gonna Give You Up (Video)"
	rickauth  = "RickAstleyVEVO"
)

var ytsrc = New(nil)

func TestSource_PlayVideo(t *testing.T) {
	track, err := ytsrc.PlayVideo(rickvid)
	_, ok := err.(ErrUnplayable)
	if ok { // if the track is unplayable skip
		t.Skip()
	}
	assert.Nil(t, err, "error is supposed to be nil")
	assert.Equal(t, ricktitle, track.Title, "the track's name should be equal")
	assert.Equal(t, rickauth, track.Author, "the track's author should be equal")
	assert.False(t, track.IsStream, "this is not a live-stream")
}

func TestSource_PlayVideoUrl(t *testing.T) {
	track, err := ytsrc.PlayVideoUrl(rick1)
	_, ok := err.(ErrUnplayable)
	if ok { // if the track is unplayable skip
		t.Skip()
	}
	assert.Nil(t, err, "error is supposed to be nil")
	assert.Equal(t, ricktitle, track.Title, "the track's name should be equal")
	assert.Equal(t, rickauth, track.Author, "the track's author should be equal")
	assert.False(t, track.IsStream, "this is not a live-stream")
}

func TestSource_PlayVideoUrl2(t *testing.T) {
	track, err := ytsrc.PlayVideoUrl(rick2)
	_, ok := err.(ErrUnplayable)
	if ok { // if the track is unplayable skip
		t.Skip()
	}
	assert.Nil(t, err, "error is supposed to be nil")
	assert.Equal(t, ricktitle, track.Title, "the track's name should be equal")
	assert.Equal(t, rickauth, track.Author, "the track's author should be equal")
	assert.False(t, track.IsStream, "this is not a live-stream")
}

func TestSource_PlayVideoUrl3(t *testing.T) {
	_, err := ytsrc.PlayVideoUrl(rick3)
	assert.NotNil(t, err, "the error is supposed to not be nil")
}

func TestSource_ExtractVideoId(t *testing.T) {
	vid, err := ytsrc.ExtractVideoId(rick1)
	assert.Nil(t, err, "error is supposed to be nil")
	assert.Equal(t, rickvid, vid, "the id should be extracted correctly")
}

func TestSource_ExtractVideoId2(t *testing.T) {
	vid, err := ytsrc.ExtractVideoId(rick2)
	assert.Nil(t, err, "error is supposed to be nil")
	assert.Equal(t, rickvid, vid, "the id should be extracted correctly")
}

func TestSource_ExtractVideoId3(t *testing.T) {
	_, err := ytsrc.ExtractVideoId(rick3)
	assert.NotNil(t, err, "the error is supposed to not be nil")
}

func TestSource_CheckVideoUrl(t *testing.T) {
	check := ytsrc.CheckVideoUrl(rick1)
	assert.True(t, check, "the url id valid")
}

func TestSource_CheckVideoUrl2(t *testing.T) {
	check := ytsrc.CheckVideoUrl(rick2)
	assert.True(t, check, "the url id valid")
}

func TestSource_CheckVideoUrl3(t *testing.T) {
	check := ytsrc.CheckVideoUrl(rick3)
	assert.False(t, check, "the url id invalid")
}

func TestTrack_Codec(t *testing.T) {
	track, err := ytsrc.PlayVideo(rickvid)
	if err != nil {
		t.Skip()
	}
	assert.Equal(t, "opus", track.Codec(), "the codec is supposed to be opus")
}
