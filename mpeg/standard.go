package mpeg

// StandardTrack stores the headers of a non-fragmented MP4 File.
type StandardTrack struct {
	*Track
	// The timescale of each track
	TimeScales map[int]int
}
