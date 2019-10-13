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
