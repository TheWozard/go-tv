package channel

import (
	"math/rand"
	"time"
)

// Schedule is an ordered list of Series with a precomputed source→position index.
// It is the read-only query layer over the content library; mutations (reorder,
// rename, apply cuts) go through the mutation package and must call RebuildIndex.
type Schedule struct {
	Series []*Series
	index  map[Source]ScheduleIndex
}

// ScheduleIndex holds the (series, season, episode) coordinates of a Source within a Schedule.
type ScheduleIndex struct {
	series  int
	season  int
	episode int
}

// NewSchedule constructs a Schedule and builds the source→index map.
func NewSchedule(series ...*Series) *Schedule {
	idx := make(map[Source]ScheduleIndex)
	for si, sr := range series {
		for sni, sn := range sr.Seasons {
			for ei, ep := range sn.Episodes {
				idx[ep.Source] = ScheduleIndex{series: si, season: sni, episode: ei}
			}
		}
	}
	return &Schedule{Series: series, index: idx}
}

// CurrentSegmentAt returns the segment that should be playing for source at position.
// If position is past all clips in the episode, it advances to the next episode
// according to the series mode. Returns false if there is nothing left to play.
func (sc *Schedule) CurrentSegmentAt(source Source, position time.Duration, shuffle bool, isActive func(string) bool) (Segment, bool) {
	idx, ok := sc.index[source]
	if !ok {
		return Segment{}, false
	}
	ep := sc.Series[idx.series].Seasons[idx.season].Episodes[idx.episode]
	if clip, ok := ep.ClipAt(position); ok {
		return Segment{Source: ep.Source, Clip: clip}, true
	}
	return sc.nextEpisodeSegment(idx, shuffle, isActive)
}

// NextSegmentAt returns the next clip boundary after position within source's episode,
// or the first segment of the next episode if no later clip exists.
func (sc *Schedule) NextSegmentAt(source Source, position time.Duration, shuffle bool, isActive func(string) bool) (Segment, bool) {
	idx, ok := sc.index[source]
	if !ok {
		return Segment{}, false
	}
	ep := sc.Series[idx.series].Seasons[idx.season].Episodes[idx.episode]
	if clip, ok := ep.ClipAfter(position); ok {
		return Segment{Source: ep.Source, Clip: clip}, true
	}
	return sc.nextEpisodeSegment(idx, shuffle, isActive)
}

func (sc *Schedule) activeSeries(isActive func(string) bool) []*Series {
	active := make([]*Series, 0, len(sc.Series))
	for _, s := range sc.Series {
		if isActive(s.ID) {
			active = append(active, s)
		}
	}
	return active
}

// nextEpisodeSegment advances past the episode at idx. When shuffle is enabled it picks
// a random active series and starts from its beginning, unless the completed
// episode has EpisodeContinueMode, which forces advancement within the current series.
func (sc *Schedule) nextEpisodeSegment(idx ScheduleIndex, shuffle bool, isActive func(string) bool) (Segment, bool) {
	sr := sc.Series[idx.series]
	ep := sr.Seasons[idx.season].Episodes[idx.episode]
	nextSeason, nextEpisode := idx.season, idx.episode+1
	if shuffle && ep.Mode != EpisodeContinueMode {
		active := sc.activeSeries(isActive)
		if len(active) == 0 {
			return Segment{}, false
		}
		sr = active[rand.Intn(len(active))]
		nextSeason, nextEpisode = 0, 0
	}
	return sr.FirstSegmentFrom(nextSeason, nextEpisode)
}

// firstActiveFrom returns the first segment of the first active series
// that appears after the series containing source in the schedule.
func (sc *Schedule) firstActiveFrom(source Source, isActive func(string) bool) (Segment, bool) {
	start := 0
	if idx, ok := sc.index[source]; ok {
		start = idx.series + 1
	}
	for i := start; i < len(sc.Series); i++ {
		sr := sc.Series[i]
		if !isActive(sr.ID) {
			continue
		}
		if seg, ok := sr.FirstSegmentFrom(0, 0); ok {
			return seg, true
		}
	}
	return Segment{}, false
}

// First returns the first playable segment across all active series.
func (sc *Schedule) First(isActive func(string) bool) (Segment, bool) {
	for _, sr := range sc.Series {
		if !isActive(sr.ID) {
			continue
		}
		if seg, ok := sr.FirstSegmentFrom(0, 0); ok {
			return seg, true
		}
	}
	return Segment{}, false
}

// SeriesOf returns the Series that contains the given source, or nil.
func (sc *Schedule) SeriesOf(src Source) *Series {
	if idx, ok := sc.index[src]; ok {
		return sc.Series[idx.series]
	}
	return nil
}

// Find is an alias for FindEpisode, returning (*Episode, bool).
func (sc *Schedule) Find(src Source) (*Episode, bool) {
	ep := sc.FindEpisode(src)
	return ep, ep != nil
}

// FindEpisode returns a pointer to the Episode that owns src, or nil.
func (sc *Schedule) FindEpisode(src Source) *Episode {
	if idx, ok := sc.index[src]; ok {
		return &sc.Series[idx.series].Seasons[idx.season].Episodes[idx.episode]
	}
	return nil
}

// RebuildIndex recomputes the source→index map after in-place series mutations.
func (sc *Schedule) RebuildIndex() {
	idx := make(map[Source]ScheduleIndex, len(sc.index))
	for si, sr := range sc.Series {
		for sni, sn := range sr.Seasons {
			for ei, ep := range sn.Episodes {
				idx[ep.Source] = ScheduleIndex{series: si, season: sni, episode: ei}
			}
		}
	}
	sc.index = idx
}
