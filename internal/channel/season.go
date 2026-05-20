package channel

// Season is an ordered collection of Episodes within a Series.
// Seasons advance sequentially; the Series mode controls what happens after the last season.
// Disabled seasons are skipped during playback iteration.
type Season struct {
	Name     string
	Episodes []Episode
	Disabled bool
}

// NewSeason constructs a named season with the given episodes.
func NewSeason(name string, episodes ...Episode) Season {
	return Season{Name: name, Episodes: episodes}
}

// NewAnonymousSeason constructs a season with no name. Used in tests.
func NewAnonymousSeason(episodes ...Episode) Season {
	return NewSeason("", episodes...)
}

// FirstSegmentFrom returns the first playable segment starting at episodeIdx.
// Episodes with no playable clips (zero-length, no clips set) are skipped.
func (s Season) FirstSegmentFrom(episodeIdx int) (Segment, bool) {
	for _, ep := range s.Episodes[min(episodeIdx, len(s.Episodes)):] {
		if clip, ok := ep.FirstClip(); ok {
			return Segment{Source: ep.Source, Clip: clip}, true
		}
	}
	return Segment{}, false
}
