package main

/*
 #cgo amd64 386 CFLAGS: -DX86=1
 #cgo LDFLAGS: -lmatroska
 #include <matroska/c/libmatroska.h>
*/
import "C"
import (
	"sync"
	"unsafe"
)

type MatroskaFile struct {
	sync.RWMutex

	Id           unsafe.Pointer
	Tracks       []C.matroska_track
	ReadResolver func() []byte
	SeekResolver func(int)
}

func Open(url string) (*MatroskaFile, error) {
	var id unsafe.Pointer = C.matroska_open_url(C.CString(url))

	amount := C.matroska_get_number_track(id)

	tracks := make([]C.matroska_track, amount)
	for i := 0; i < amount; i++ {
		tracks = append(tracks, C.matroska_get_track(id, uint8(i)))
	}

	return &MatroskaFile{
		RWMutex:      sync.RWMutex{},
		Tracks:       tracks,
		Id:           id,
		ReadResolver: nil,
		SeekResolver: nil,
	}, nil
}

func (track *MatroskaFile) Close() {
	C.matroksa_close(track.Id)
}
