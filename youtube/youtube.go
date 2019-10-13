package youtube

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var cipherCache sync.Map = sync.Map{}

var (
	CipherVar    = "[a-zA-Z_\\$][a-zA-Z_0-9]*"
	CipherVarDef = "\\\"?" + CipherVar + "\\\"?"
	CipherBefAcc = "(?:\\[\\\"|\\.)"
	CipherAftAcc = "(?:\\\"\\]|)"
	CipherVarAcc = CipherBefAcc + CipherVar + CipherAftAcc
	CipherRev    = ":function\\(a\\)\\{(?:return )?a\\.reverse\\(\\)\\}"
	CipherSlice  = ":function\\(a,b\\)\\{return a\\.slice\\(b\\)\\}"
	CipherSplice = ":function\\(a,b\\)\\{a\\.splice\\(0,b\\)\\}"
	CipherSwap   = ":function\\(a,b\\)\\{var c=a\\[0\\];a\\[0\\]=a\\[b%a\\.length\\];a" +
		"\\[b(?:%a.length|)\\]=c(?:;return a)?\\}"
	CipherFunc, _ = regexp.Compile("function(?: " + CipherVar + ")?\\(a\\)\\{" +
		"a=a\\.split\\(\"\"\\);\\s*" +
		"((?:(?:a=)?" + CipherVar + CipherVarAcc + "\\(a,\\d+\\);)+)" +
		"return a\\.join\\(\"\"\\)" +
		"\\}")
	CipherAct, _ = regexp.Compile(
		"var (" + CipherVar + ")=\\{((?:(?:" +
			CipherVarDef + CipherRev + "|" +
			CipherVarDef + CipherSlice + "|" +
			CipherVarDef + CipherSplice + "|" +
			CipherVarDef + CipherSwap +
			"),?\\n?)+)\\};")
)

type YoutubeFormat struct {
	Type         string
	Bitrate      int64
	Clen         int64
	Url          string
	Signature    string
	SignatureKey string
	PlayerScript string
}

func compileAndExtract(pattern, body string) (string, error) {
	p, err := regexp.Compile("(?m:^|,)\\\"?(" + CipherVar + ")\\\"?" + pattern)
	if err != nil {
		return "", err
	}
	s := p.FindStringSubmatch(body)
	if s == nil {
		return "", nil
	}
	return strings.ReplaceAll(s[1], "$", "\\$"), nil
}

func (ytfmt *YoutubeFormat) GetValidUrl() (string, error) {
	if ytfmt.Signature == "" {
		return ytfmt.Url, nil
	}
	cache, ok := cipherCache.Load(ytfmt.PlayerScript)
	if ok {
		return cache.(string), nil
	}
	cipher, err := http.Get("https://s.ytimg.com" + ytfmt.PlayerScript)
	if err != nil {
		return "", err
	}
	x, err := ioutil.ReadAll(cipher.Body)
	if err != nil {
		return "", err
	}
	actions := CipherAct.FindStringSubmatch(string(x))
	if len(actions) == 0 {
		return "", errors.New("unable to decipher: couldn't find any submatches")
	}
	actionbody := actions[2]

	funcs := make([]string, 0)
	revkey, err := compileAndExtract(CipherRev, actionbody)
	if err != nil {
		return "", err
	}
	if revkey != "" {
		funcs = append(funcs, regexp.QuoteMeta(revkey))
	}
	slicekey, err := compileAndExtract(CipherSlice, actionbody)
	if err != nil {
		return "", err
	}
	if slicekey != "" {
		funcs = append(funcs, regexp.QuoteMeta(slicekey))
	}
	splicekey, err := compileAndExtract(CipherSplice, actionbody)
	if err != nil {
		return "", err
	}
	if splicekey != "" {
		funcs = append(funcs, regexp.QuoteMeta(splicekey))
	}
	swapkey, err := compileAndExtract(CipherSwap, actionbody)
	if err != nil {
		return "", err
	}
	if swapkey != "" {
		funcs = append(funcs, regexp.QuoteMeta(swapkey))
	}
	extractor, err := regexp.Compile("(?:a=)?" + regexp.QuoteMeta(actions[1]) + CipherBefAcc + "(" +
		strings.Join(funcs, "|") +
		")" + CipherAftAcc + "\\(a,(\\d+)\\)")
	if err != nil {
		return "", err
	}
	functions := CipherFunc.FindStringSubmatch(string(x))
	if len(functions) == 0 {
		return "", errors.New("can't find decipher")
	}
	submatches := extractor.FindAllStringSubmatch(functions[1], -1)

	tempurl := []byte(ytfmt.Signature)

	for _, match := range submatches {
		t := match[1]
		switch t {
		case swapkey:
			{
				pos, err := strconv.Atoi(match[2])
				if err != nil {
					continue
				}
				tempurl[pos%len(tempurl)], tempurl[0] = tempurl[0], tempurl[pos%len(tempurl)]

			}
		case revkey:
			{
				for i, j := 0, len(tempurl)-1; i < j; i, j = i+1, j-1 {
					tempurl[i], tempurl[j] = tempurl[j], tempurl[i]
				}
			}
		case slicekey, splicekey:
			{
				pos, err := strconv.Atoi(match[2])
				if err != nil {
					continue
				}
				tempurl = tempurl[pos:]
			}
		}
	}
	uri, err := url.Parse(ytfmt.Url)
	if err != nil {
		return "", err
	}
	q := uri.Query()
	q.Add("ratebypass", "yes")
	q.Add(ytfmt.SignatureKey, string(tempurl))
	uri.RawQuery = q.Encode()
	return uri.String(), nil
}

type YoutubeTrack struct {
	VideoId  string
	Title    string
	Author   string
	Duration time.Duration
	IsStream bool
	Format   *YoutubeFormat
}

type YoutubeSource struct {
	Client http.Client
}

func NewYoutubeSource() *YoutubeSource {
	return &YoutubeSource{Client: http.Client{}}
}

// Currently supports only audio/webm with the opus codec.
// It will support more codecs in the future
func findBestFormat(args map[string]interface{}, js string) (*YoutubeFormat, error) {
	adpt := strings.Split(args["adaptive_fmts"].(string), ",")

	var bestformat *YoutubeFormat
	for _, format := range adpt {
		fomt, err := url.ParseQuery(format)
		if err != nil {
			continue
		}
		if strings.HasPrefix(fomt.Get("type"), "video/") {
			continue
		}
		if bitrate, err := strconv.Atoi(fomt.Get("bitrate")); err == nil && (bestformat == nil || int64(bitrate) > bestformat.Bitrate) {
			clen, err := strconv.Atoi(fomt.Get("bitrate"))
			if err != nil {
				continue
			}
			sk := fomt.Get("sp")
			if sk == "" {
				sk = "signature"
			}
			bestformat = &YoutubeFormat{
				Type:         fomt.Get("type"),
				Bitrate:      int64(bitrate),
				Clen:         int64(clen),
				Url:          fomt.Get("url"),
				Signature:    fomt.Get("s"),
				SignatureKey: sk,
				PlayerScript: js,
			}
		}
	}
	return bestformat, nil
}

func (yt YoutubeSource) PlayVideo(videoId string) (*YoutubeTrack, error) {
	req, err := http.NewRequest("GET", strings.Join([]string{"https://www.youtube.com/watch?v=", videoId, "&pbj=1&hl=en"}, ""), nil)
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
		return &YoutubeTrack{
			VideoId:  videoId,
			Title:    vDetails["title"].(string),
			Author:   vDetails["author"].(string),
			Duration: duration,
			IsStream: isStream,
			Format:   format,
		}, nil
	} else {
		return nil, errors.New("couldn't find the track")
	}
}
