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

// Package youtube abstracts searching and playing audio through youtube.
//
// Currently only the audio/webm container and the opus codec are supported.
package youtube

import (
	"encoding/json"
	"errors"
	"github.com/dondish/lionplayer/core"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// Source is an HTTP
type Source struct {
	Client *http.Client
}

// New creates a new Source using the Client given, if nil, it will use the default one.
func New(client *http.Client) *Source {
	if client == nil {
		return &Source{Client: core.DefaultHTTPClient}
	}
	return &Source{Client: client}
}

// PlayVideo plays a video using the video id given.
// Returns a youtube track that implements core.Track
func (yt Source) PlayVideo(videoId string) (*Track, error) {
	req, err := http.NewRequest("GET", "https://www.youtube.com/watch?v="+videoId+"&pbj=1&hl=en", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "lionPlayer v0.1")
	req.Header.Add("X-YouTube-Client-Name", "1")
	req.Header.Add("X-YouTube-Client-Version", "2.20191008.04.01")
	res, err := yt.Client.Do(req)
	if err != nil {
		return nil, err
	}
	var resjson []interface{}
	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&resjson)
	if err != nil {
		return nil, err
	}
	var pi map[string]interface{}
	for _, arg := range resjson {
		if r, ok := arg.(map[string]interface{})["player"]; ok {
			pi = r.(map[string]interface{})
			break
		}
	}
	if pi == nil {
		return nil, errors.New("pi is nil")
	}
	args := pi["args"].(map[string]interface{})
	playerResponse, ok := args["player_response"]

	if ok {
		var pres map[string]interface{}
		err = json.Unmarshal([]byte(playerResponse.(string)), &pres)
		if err != nil {
			return nil, err
		}
		vDetails := pres["videoDetails"].(map[string]interface{})
		isStream := vDetails["isLiveContent"].(bool)
		var duration time.Duration
		if isStream {
			duration = math.MaxInt64
		} else {
			seconds, err := strconv.Atoi(vDetails["lengthSeconds"].(string))
			if err != nil {
				return nil, err
			}
			duration = time.Second * time.Duration(seconds)
		}
		format, err := findBestFormat(args, pi["assets"].(map[string]interface{})["js"].(string))
		if err != nil {
			return nil, err
		}
		return &Track{
			VideoId:  videoId,
			Title:    vDetails["title"].(string),
			Author:   vDetails["author"].(string),
			Length:   duration,
			IsStream: isStream,
			Format:   format,
			source:   &yt,
		}, nil
	} else {
		return nil, errors.New("couldn't find the track")
	}
}

var (
	WatchUrl, _ = regexp.Compile("(?:https?://)?(?:www\\.)?(?:youtu\\.be/|youtube\\.com(?:/embed/|/v/|/watch.+v=))([\\w-_]{10,12})(?: [^\"& ]+)?")
)

// PlayVideoUrl plays a video using the video url given.
// Returns a youtube track that implements core.Track
//
// Internally it extracts the videoID and calls PlayVideo
func (yt Source) PlayVideoUrl(videoUrl string) (*Track, error) {
	matches := WatchUrl.FindStringSubmatch(videoUrl)
	if len(matches) >= 2 {
		return yt.PlayVideo(matches[1])
	}
	return nil, errors.New("unable to extract the video id")
}

// Extracts the VideoId out of the URL
func (yt Source) ExtractVideoId(videoUrl string) (string, error) {
	matches := WatchUrl.FindStringSubmatch(videoUrl)
	if len(matches) >= 2 {
		return matches[1], nil
	}
	return "", errors.New("unable to extract the video id")
}

// Checks whether a video URL is valid
func (yt Source) CheckVideoUrl(videoUrl string) bool {
	return WatchUrl.MatchString(videoUrl)
}
