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
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

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

var cipherCache = sync.Map{}

// Defines a youtube format
type Format struct {
	Type         string // The audio codec and container used
	Bitrate      int64  // The bitrate
	Clen         int64  // The length of the content
	Url          string // The direct URL (without signature)
	Signature    string // The signature
	SignatureKey string // The key in the query for the signature
	PlayerScript string // The location of the JS to parse the signature
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

// Decipher the signature if exists to get the valid url.
func (ytfmt *Format) GetValidUrl() (string, error) {
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

// Currently supports only audio/webm with the opus codec.
// It will support more codecs in the future
func findBestFormat(args map[string]interface{}, js string) (*Format, error) {
	adpt := strings.Split(args["adaptive_fmts"].(string), ",")

	var bestformat *Format
	for _, format := range adpt {
		fomt, err := url.ParseQuery(format)
		if err != nil {
			continue
		}
		if strings.HasPrefix(fomt.Get("type"), "video/") || !strings.Contains(fomt.Get("type"), "webm") {
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
			bestformat = &Format{
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
