package channel

import (
	"context"
	"errors"
	"sort"
	"time"
)

var (
	ErrEpisodeNotInSchedule = errors.New("episode not in schedule")
	ErrSeriesNotFound       = errors.New("series not found")
	ErrNoNextFragment       = errors.New("no next fragment")
)

// Channel coordinates schedule and state for a single channel.
type Channel struct {
	schedule *Schedule
	state    *State
}

// NewChannel wraps a schedule and state into a Channel.
func NewChannel(schedule *Schedule, state *State) *Channel {
	return &Channel{schedule: schedule, state: state}
}

// CurrentState returns the active source, playback position, and stop point.
func (c *Channel) CurrentState() (source Source, position, stopAt time.Duration, ok bool) {
	src, pos := c.state.Get()
	frag, ok := c.schedule.Current(src, pos)
	if frag.Source.Equal(src) {
		frag.Start = pos
	}
	return frag.Source, frag.Start, frag.End, ok
}

// AllSeries returns all series in the schedule.
func (c *Channel) AllSeries() []*Series {
	return c.schedule.AllSeries()
}

// Progress records playback position. Ignored if source is stale.
func (c *Channel) Progress(source Source, position time.Duration) {
	c.state.SetPosition(source, position)
	if ser := c.schedule.SeriesOf(source); ser != nil {
		ser.UpdateState(source, position)
	}
}

// Jump unconditionally moves playback to source at position.
func (c *Channel) Jump(source Source, position time.Duration) error {
	if _, ok := c.schedule.Find(source); !ok {
		return ErrEpisodeNotInSchedule
	}
	c.state.Jump(source, position)
	if ser := c.schedule.SeriesOf(source); ser != nil {
		ser.JumpState(source, position)
	}
	return nil
}

// Next advances playback past source to the next fragment.
func (c *Channel) Next(source Source, position time.Duration) error {
	frag, ok := c.schedule.Next(source, position)
	if !ok {
		return ErrNoNextFragment
	}
	c.state.Advance(source, frag.Source, frag.Start)
	if ser := c.schedule.SeriesOf(frag.Source); ser != nil {
		ser.JumpState(frag.Source, frag.Start)
	}
	return nil
}

// RenameSeason renames a season within the named series.
func (c *Channel) RenameSeason(seriesName, oldName, newName string) error {
	ser := c.schedule.FindSeries(seriesName)
	if ser == nil {
		return ErrSeriesNotFound
	}
	seasons := ser.AllSeasons()
	for i, s := range seasons {
		if s.Name == oldName {
			seasons[i].Name = newName
			break
		}
	}
	ser.UpdateSeasons(seasons)
	return ser.Save()
}

// SeasonOrder describes a season's desired episode ordering.
type SeasonOrder struct {
	Name       string
	EpisodeIDs []string
}

// ReorderSeries updates the season and episode order within a series and saves.
func (c *Channel) ReorderSeries(seriesName string, orders []SeasonOrder) error {
	ser := c.schedule.FindSeries(seriesName)
	if ser == nil {
		return ErrSeriesNotFound
	}

	existing := make(map[string]Episode)
	for _, season := range ser.AllSeasons() {
		for _, ep := range season.Episodes {
			existing[ep.Source.ID] = ep
		}
	}

	seasons := make([]Season, len(orders))
	for i, o := range orders {
		episodes := make([]Episode, 0, len(o.EpisodeIDs))
		for _, id := range o.EpisodeIDs {
			if ep, ok := existing[id]; ok {
				episodes = append(episodes, ep)
			}
		}
		seasons[i] = Season{Name: o.Name, Episodes: episodes}
	}
	ser.UpdateSeasons(seasons)
	c.schedule.rebuildIndex()
	return ser.Save()
}

// CutRange is a time range to remove from an episode.
type CutRange struct {
	Start time.Duration
	End   time.Duration
}

// ApplyCuts converts cut ranges into keep-segments, updates the schedule, and saves.
// Returns the updated Episode or an error if the episode is not found or save fails.
func (c *Channel) ApplyCuts(videoID string, cuts []CutRange) (*Episode, error) {
	episode, ok := c.schedule.Find(NewYoutubeSource(videoID))
	if !ok {
		return nil, ErrEpisodeNotInSchedule
	}

	const minDur = 10 * time.Second
	var newSegments []Segment

	if len(cuts) > 0 {
		sort.Slice(cuts, func(i, j int) bool { return cuts[i].Start < cuts[j].Start })
		merged := []CutRange{cuts[0]}
		for _, cr := range cuts[1:] {
			last := &merged[len(merged)-1]
			if cr.Start <= last.End {
				if cr.End > last.End {
					last.End = cr.End
				}
			} else {
				merged = append(merged, cr)
			}
		}
		pos := time.Duration(0)
		for _, cut := range merged {
			if cut.Start-pos >= minDur {
				newSegments = append(newSegments, makeSeg(pos, cut.Start))
			}
			pos = cut.End
		}
		if episode.Length.Duration-pos >= minDur {
			newSegments = append(newSegments, makeSeg(pos, episode.Length.Duration))
		}
		v := &Episode{Segments: newSegments, Length: episode.Length}
		v.Clean()
		newSegments = v.Segments
	}

	// Find the series that contains this episode and update it.
	for _, ser := range c.schedule.AllSeries() {
		seasons := ser.AllSeasons()
		changed := false
		for i, season := range seasons {
			for j, ep := range season.Episodes {
				if ep.Source.ID == videoID {
					seasons[i].Episodes[j].Segments = newSegments
					changed = true
				}
			}
		}
		if changed {
			ser.UpdateSeasons(seasons)
			if err := ser.Save(); err != nil {
				return nil, err
			}
			break
		}
	}

	updated, _ := c.schedule.Find(NewYoutubeSource(videoID))
	return updated, nil
}

// SaveState persists the current playback state and all per-series states to disk.
func (c *Channel) SaveState() error {
	if err := c.state.Save(); err != nil {
		return err
	}
	for _, ser := range c.schedule.AllSeries() {
		if err := ser.SaveState(); err != nil {
			return err
		}
	}
	return nil
}

// StartAutoSave saves state on interval until ctx is cancelled.
func (c *Channel) StartAutoSave(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = c.state.Save()
				for _, ser := range c.schedule.AllSeries() {
					_ = ser.SaveState()
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func makeSeg(start, end time.Duration) Segment {
	seg := Segment{}
	if start > 0 {
		d := Duration{Duration: start}
		seg.Start = &d
	}
	if end > 0 {
		d := Duration{Duration: end}
		seg.End = &d
	}
	return seg
}
