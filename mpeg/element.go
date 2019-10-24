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

package mpeg

import (
	"golang.org/x/text/encoding/charmap"
	"io"
)

var (
	isodec = charmap.ISO8859_1.NewDecoder()
)

type Element struct {
	R       io.ReadSeeker
	N       int64
	Offset  int64
	Id      string
	intbuf  []byte
	Version int32
	Flags   int32
}

func (e *Element) readType() (string, error) {
	_, err := io.ReadFull(e.R, e.intbuf)
	if err != nil {
		return "", err
	}
	con, err := isodec.Bytes(e.intbuf)
	if err != nil {
		return "", err
	}
	return string(con), nil
}

func parseInt(data []byte) uint64 {
	var in uint64
	for i := 0; i < len(data); i++ {
		in <<= 8
		in |= uint64(data[i])
	}
	return in
}

func (e *Element) readInt64() (uint64, error) {
	n, err := io.ReadFull(e.R, e.intbuf)
	if n == 1 && e.intbuf[0] == 255 {
		return 0, io.EOF
	}
	if err != nil {
		return 0, err
	}
	return parseInt(e.intbuf), nil
}

func (e *Element) readInt32() (uint32, error) {
	_, err := io.ReadFull(e.R, e.intbuf[:4])
	if err != nil {
		return 0, err
	}
	return uint32(parseInt(e.intbuf[:4])), nil
}

func (e *Element) readInt16() (uint16, error) {
	_, err := io.ReadFull(e.R, e.intbuf[:2])
	if err != nil {
		return 0, err
	}
	return uint16(parseInt(e.intbuf[:2])), nil
}

func (e *Element) Next() (*Element, error) {
	offs, err := e.R.Seek(0, 1)
	if err != nil {
		return nil, err
	}
	leng, err := e.readInt64()
	if err != nil {
		return nil, err
	}
	typ, err := e.readType()
	return &Element{
		R:      newLimitedReadSeeker(e.R, int64(leng)),
		N:      int64(leng),
		Offset: offs,
		Id:     typ,
		intbuf: e.intbuf,
	}, nil
}

func (e *Element) Skip() error {
	_, err := e.R.Seek(e.Offset+e.N, 0)
	return err
}

func (e *Element) ParseFlags() error {
	in, err := e.readInt32()
	if err != nil {
		return err
	}
	e.Version = int32(in << 24)
	e.Flags = int32(in & 0xffffff)
	return nil
}
