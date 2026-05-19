package channel

import (
	"crypto/rand"
	"encoding/hex"
)

// SeriesMode controls what happens when playback reaches the end of the last season.
type SeriesMode string

const (
	// OnceMode is the default mode (zero value). Playback stops after the last season.
	OnceMode SeriesMode = ""
	// LoopMode wraps back to the first season after the last season finishes.
	LoopMode SeriesMode = "loop"
)

// Series is a named collection of Seasons with a shared playback mode.
// ID is a randomly generated identifier that persists across renames.
type Series struct {
	ID      string
	Name    string
	Mode    SeriesMode
	Seasons []Season
}

// NewSeriesID generates a random series identifier.
func NewSeriesID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewSeries constructs a Series with a randomly generated ID.
func NewSeries(name string, mode SeriesMode, seasons ...Season) *Series {
	return &Series{ID: NewSeriesID(), Name: name, Mode: mode, Seasons: seasons}
}

// NewSeriesWithID constructs a Series with an explicit ID.
// Used by the store when deserializing persisted data.
func NewSeriesWithID(id, name string, mode SeriesMode, seasons ...Season) *Series {
	return &Series{ID: id, Name: name, Mode: mode, Seasons: seasons}
}

// NewAnonymousSeries constructs a Series with no name. Used in tests.
func NewAnonymousSeries(mode SeriesMode, seasons ...Season) *Series {
	return NewSeries("anonymous", mode, seasons...)
}

// FirstSegmentFrom returns the first playable segment starting from (seasonIdx, episodeIdx).
//
// In LoopMode, once the remaining seasons are exhausted the search wraps back to
// episode 0 of season 0. In all other modes the search stops at the last season.
func (sr *Series) FirstSegmentFrom(seasonIdx, episodeIdx int) (Segment, bool) {
	start := min(seasonIdx, len(sr.Seasons))
	for _, s := range sr.Seasons[start:] {
		if seg, ok := s.FirstSegmentFrom(episodeIdx); ok {
			return seg, true
		}
		episodeIdx = 0
	}
	if sr.Mode == LoopMode {
		for _, s := range sr.Seasons {
			if seg, ok := s.FirstSegmentFrom(0); ok {
				return seg, true
			}
		}
	}
	return Segment{}, false
}
