package channel

import (
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
// within the same series. Returns false if there is nothing left to play.
func (sc *Schedule) CurrentSegmentAt(source Source, position time.Duration) (Segment, bool) {
	idx, ok := sc.index[source]
	if !ok {
		return Segment{}, false
	}
	ep := sc.Series[idx.series].Seasons[idx.season].Episodes[idx.episode]
	if clip, ok := ep.ClipAt(position); ok {
		return Segment{Source: ep.Source, Clip: clip}, true
	}
	return sc.Series[idx.series].FirstSegmentFrom(idx.season, idx.episode+1)
}

// NextEpisodeInSeries returns the first segment of the episode immediately after
// source's episode, staying within the same series. Returns false if source is not
// in the schedule or no next episode exists (accounting for loop mode).
func (sc *Schedule) NextEpisodeInSeries(source Source) (Segment, bool) {
	idx, ok := sc.index[source]
	if !ok {
		return Segment{}, false
	}
	return sc.Series[idx.series].FirstSegmentFrom(idx.season, idx.episode+1)
}

// ActiveSeries returns all series for which isActive returns true.
func (sc *Schedule) ActiveSeries(isActive func(string) bool) []*Series {
	active := make([]*Series, 0, len(sc.Series))
	for _, s := range sc.Series {
		if isActive(s.ID) {
			active = append(active, s)
		}
	}
	return active
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
