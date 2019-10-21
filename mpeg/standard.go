package mpeg

// Non-Fragmented MP4 File
type StandardTrack struct {
	*Track
	TimeScales map[int]int // The timescale of each track
}
