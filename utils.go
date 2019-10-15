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

// Package main is responsible for launching the bot.
package main

import (
	"sync"
)

// The listener for the EventListener
type Listener struct {
	Filter func(interface{}) bool // The filter to run to check whether to execute the action
	Action func(interface{})      // The function to run otherwise
}

// The event listener consumes the channel, so it should not be used in parallel
type EventListener struct {
	Channel    chan interface{}
	Listeners  []Listener
	WaitingFor sync.Map
}

// Starts the listener, from now on don't use the channel
func (e EventListener) StartListener() {
	for v := range e.Channel {
		for _, list := range e.Listeners {
			if list.Filter(v) {
				list.Action(v)
			}
		}
		e.WaitingFor.Range(func(filter, action interface{}) bool {
			if filter.(func(interface{}) bool)(v) {
				action.(chan interface{}) <- v
				close(action.(chan interface{}))
				e.WaitingFor.Delete(filter)
			}
			return true
		})
	}
	e.close() // When the channel closes, automatically clears the listeners
}

// Adds another listener
func (e *EventListener) AddListener(f Listener) {
	e.Listeners = append(e.Listeners, f)
}

// Returns a channel that returns the first item that satisfies the requirement
func (e *EventListener) WaitFor(f func(interface{}) bool) <-chan interface{} {
	c := make(chan interface{})
	e.WaitingFor.Store(f, c)
	return c
}

// Clears out the event listener when the channel closes
func (e *EventListener) close() {
	e.WaitingFor.Range(func(filter, action interface{}) bool {
		close(action.(chan interface{}))
		e.WaitingFor.Delete(filter)
		return true
	})
	e.Listeners = []Listener{}
}

//type SeekableBufferedReader struct {
//	io.Reader
//	buf bytes.Buffer
//}
//
//func (s SeekableBufferedReader) Seek(offset int64, whence int) (int64, error) {
//	s.buf.Truncate()
//}
