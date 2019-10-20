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

package mp4

import (
	"fmt"
	"io"
	"log"
)

/**
Taken from https://github.com/ebml-go/ebml
No need for a dependency just for the reader
*/

type limitedReadSeeker struct {
	*io.LimitedReader
}

func newLimitedReadSeeker(rs io.ReadSeeker, limit int64) *limitedReadSeeker {
	return &limitedReadSeeker{&io.LimitedReader{rs, limit}}
}

func (lrs *limitedReadSeeker) String() string {
	return fmt.Sprintf("%+v", lrs.LimitedReader)
}

func (lrs *limitedReadSeeker) Seek(offset int64, whence int) (ret int64, err error) {
	//	log.Println("seek0", lrs, offset, whence)
	s := lrs.LimitedReader.R.(io.Seeker)
	prevN := lrs.LimitedReader.N
	var curr int64
	curr, err = s.Seek(0, 1)
	if err != nil {
		log.Panic(err)
	}
	ret, err = s.Seek(offset, whence)
	if err != nil {
		log.Panic(err)
	}
	lrs.LimitedReader.N += curr - ret
	if offset == 0 && whence == 1 {
		if lrs.LimitedReader.N != prevN {
			log.Panic(lrs.LimitedReader.N, prevN, curr, ret)
		}
	}
	//	log.Println("seek1", lrs, offset, whence)
	return
}
