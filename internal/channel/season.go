package channel

import "iter"

type Season struct {
	Episodes []Episode
}

func NewSeason(episodes ...Episode) Season {
	return Season{Episodes: episodes}
}

func (s Season) IterSegmentsFrom(episodeIdx int) iter.Seq[Segment] {
	return func(yield func(Segment) bool) {
		for _, ep := range s.Episodes[min(episodeIdx, len(s.Episodes)):] {
			for clip := range ep.IterClips() {
				if !yield(NewSegment(ep.source, clip)) {
					return
				}
			}
		}
	}
}
