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

/*
Reimplementation of https://github.com/jeffallen/seekinghttp/ with buffered readers and smart seeking.
*/
package seekablehttp

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/dondish/lionplayer/core"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
)

// SeekingHTTP uses a series of HTTP GETs with Range headers
// to implement io.ReadSeeker and io.ReaderAt.
type SeekingHTTP struct {
	URL     string       // The URL to connect to
	Client  *http.Client // The HTTP DefaultHTTPClient to use (allows of client reuse)
	url     *url.URL     // The url.URL representation of SeekingHTTP.Url
	offset  int64
	resp    io.ReadCloser
	respbuf *bufio.Reader
	length  int64
	open    bool
}

// Closes the client and frees up resources
func (s *SeekingHTTP) Close() error {
	s.respbuf = nil
	s.Client = nil
	s.url = nil
	return s.close()
}

// Internal connection closing used for seeks above the current buffer limit
func (s *SeekingHTTP) close() error {
	if s.open {
		core.ReleaseBufferedReader(s.respbuf)
		s.open = false
		return s.resp.Close()
	}
	return nil
}

// Compile-time check of interface implementations.
var _ io.ReadSeeker = (*SeekingHTTP)(nil)
var _ io.ReaderAt = (*SeekingHTTP)(nil)

// New initializes a SeekingHTTP for the given URL.
// The SeekingHTTP.DefaultHTTPClient field may be set before the first call
// to Read or Seek.
func New(url string, length int64) *SeekingHTTP {
	return &SeekingHTTP{
		URL:    url,
		offset: 0,
		length: length,
	}
}

// Creates a new requests with the built template
func (s *SeekingHTTP) newreq() (*http.Request, error) {
	var err error
	if s.url == nil {
		s.url, err = url.Parse(s.URL)
		if err != nil {
			return nil, err
		}
	}
	return &http.Request{
		Method:     "GET",
		URL:        s.url,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       nil,
		Host:       s.url.Host,
	}, nil
}

// Formats the range header
func fmtRange(from int64) string {
	return fmt.Sprintf("bytes=%v-", from)
}

// ReadAt reads len(buf) bytes into buf starting at offset off.
func (s *SeekingHTTP) ReadAt(buf []byte, off int64) (int, error) {
	if !s.open {
		req, err := s.newreq()
		if err != nil {
			return 0, err
		}

		rng := fmtRange(off)
		req.Header.Add("Range", rng)

		if err := s.init(); err != nil {
			return 0, err
		}
		resp, err := s.Client.Do(req)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
			s.resp = resp.Body
			s.open = true
			s.respbuf = core.AcquireBufferedReader(s.resp)
		}
	}
	return s.respbuf.Read(buf)
}

// If they did not give us an HTTP Client, use the default one.
func (s *SeekingHTTP) init() error {
	if s.Client == nil {
		s.Client = core.DefaultHTTPClient
	}

	return nil
}

// ReadAt reads len(buf) bytes into buf
func (s *SeekingHTTP) Read(buf []byte) (int, error) {
	n, err := s.ReadAt(buf, s.offset)
	if err == nil {
		s.offset += int64(n)
	}

	return n, err
}

// Seek sets the offset for the next Read.
func (s *SeekingHTTP) Seek(offset int64, whence int) (int64, error) {
	oldoff := s.offset
	switch whence {
	case io.SeekStart:
		s.offset = offset
	case io.SeekCurrent:
		s.offset += offset
	case io.SeekEnd:
		if s.length != math.MaxInt64 {
			s.offset = s.length - s.offset
		} else {
			return 0, errors.New("no seek end in a stream")
		}
	default:
		return 0, os.ErrInvalid
	}
	if s.offset == oldoff {
		return s.offset, nil
	}
	if s.offset > oldoff && s.offset-oldoff < int64(s.respbuf.Buffered()) {
		_, err := s.respbuf.Discard(int(s.offset - oldoff))
		if err != nil {
			return 0, err
		}
	} else {
		err := s.close()
		if err != nil {
			return 0, err
		}
		if s.length == math.MaxInt64 {
			return 0, nil
		}
	}
	return s.offset, nil
}

// Size uses an HTTP HEAD to find out how many bytes are available in total.
func (s *SeekingHTTP) Size() (int64, error) {
	if err := s.init(); err != nil {
		return 0, err
	}

	req, err := s.newreq()
	if err != nil {
		return 0, err
	}
	req.Method = "HEAD"

	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.ContentLength < 0 {
		return 0, errors.New("no content length for Size()")
	}

	return resp.ContentLength, nil
}
