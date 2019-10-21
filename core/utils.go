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

package core

import (
	"bufio"
	"io"
	"net/http"
	"sync"
	"time"
)

// The default http client
// Prevents creation of multiple clients by default
var DefaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// A struct that implements io.Reader that does nothing
// Should not be called and it is used for pooled buffers.
type noopReader struct {
}

var noopSingleton = &noopReader{}

func (n2 noopReader) Read(p []byte) (n int, err error) {
	return 0, nil
}

// A pool of buffered readers
var bufReaderPool = &sync.Pool{
	New: func() interface{} {
		return bufio.NewReader(noopSingleton)
	},
}

// Acquire a new bufio.Reader from the pool
// This method of reader reuse prevents GC pressure and maintains low memory overhead
func AcquireBufferedReader(reader io.Reader) *bufio.Reader {
	buf := bufReaderPool.Get().(*bufio.Reader)
	buf.Reset(reader)
	return buf
}

// Return a bufio.Reader to the pool
// This method of reader reuse prevents GC pressure and maintains low memory overhead
func ReleaseBufferedReader(buf *bufio.Reader) {
	buf.Reset(noopSingleton)
	bufReaderPool.Put(buf)
}
