package channel

import (
	"math/rand"
	"time"
)

type Schedule struct {
	series []Series
	state  State
	index  map[Source]ScheduleIndex
}

type ScheduleIndex struct {
	series  int
	season  int
	episode int
}

func NewSchedule(series ...Series) Schedule {
	idx := make(map[Source]ScheduleIndex)
	for si, sr := range series {
		for sni, sn := range sr.Seasons {
			for ei, ep := range sn.Episodes {
				idx[ep.source] = ScheduleIndex{series: si, season: sni, episode: ei}
			}
		}
	}
	return Schedule{series: series, index: idx}
}

func (sc Schedule) CurrentSegmentAt(source Source, position time.Duration) (Segment, bool) {
	idx, ok := sc.index[source]
	if !ok {
		idx = ScheduleIndex{}
	}
	sr := sc.series[idx.series]
	ep := sr.Seasons[idx.season].Episodes[idx.episode]
	for clip := range ep.IterClips() {
		if position < clip.End() {
			return NewSegment(ep.source, clip), true
		}
	}
	return sc.nextEpisodeSegment(idx, ep.mode)
}

func (sc Schedule) NextSegmentAt(source Source, position time.Duration) (Segment, bool) {
	idx, ok := sc.index[source]
	if !ok {
		return Segment{}, false
	}
	sr := sc.series[idx.series]
	ep := sr.Seasons[idx.season].Episodes[idx.episode]
	for clip := range ep.IterClips() {
		if clip.Start() > position {
			return NewSegment(ep.source, clip), true
		}
	}
	return sc.nextEpisodeSegment(idx, ep.mode)
}

func (sc Schedule) nextEpisodeSegment(idx ScheduleIndex, mode EpisodeMode) (Segment, bool) {
	sr := sc.series[idx.series]
	if mode == ShuffleEpisodeMode && len(sr.Seasons) > 0 {
		si := rand.Intn(len(sc.series))
		sr = sc.series[si]
		if state, ok := sc.state.Get(sr); ok {
			idx = sc.index[state.Source]
		} else {
			idx = ScheduleIndex{
				series:  si,
				season:  0,
				episode: 0,
			}
		}

	}
	for seg := range sr.IterSegmentsFrom(idx.season, idx.episode+1) {
		return seg, true
	}
	return Segment{}, false
}

func (sc Schedule) UpdateState(segment Segment, position time.Duration) {
	if idx, ok := sc.index[segment.Source()]; ok {
		sr := sc.series[idx.series]
		sc.state.Update(sr, segment.source, position)
	}
}
