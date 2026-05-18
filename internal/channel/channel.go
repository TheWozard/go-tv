package channel

import (
	"errors"
	"time"
)

// Channel is the runtime orchestrator that pairs a Schedule with a playback State.
// It exposes the four core player operations — CurrentSegment, Progress, Next, Jump —
// and is the only entry point for mutating state.
//
// Channel contains only domain logic. Persistence (loading from disk, auto-save)
// is handled by store.ChannelStore.
type Channel struct {
	schedule *Schedule
	state    *State
}

// NewChannel constructs a Channel from an existing schedule and state.
func NewChannel(schedule *Schedule, state *State) *Channel {
	return &Channel{
		schedule: schedule,
		state:    state,
	}
}

// NewEmptyChannel constructs a Channel with a fresh, empty state (no history, all series active).
func NewEmptyChannel(schedule *Schedule) *Channel {
	return NewChannel(schedule, NewEmptyState())
}

// CurrentSegment returns the segment that should currently be playing.
// If no state has been recorded yet, it falls back to the first segment of the
// first active series. If the saved position is no longer valid, it falls back
// to the same default.
func (c *Channel) CurrentSegment() Segment {
	src, pos := c.state.Get()
	if src.IsZero() {
		seg, _ := c.schedule.First(c.state.IsActive)
		return seg
	}
	seg, ok := c.schedule.CurrentSegmentAt(src, pos, c.state.Shuffle, c.state.IsActive)
	if !ok {
		seg, _ = c.schedule.First(c.state.IsActive)
	}
	return seg
}

// Progress records the current playback position without advancing.
// Called periodically by the player to persist where the viewer is within the episode.
func (c *Channel) Progress(source Source, position time.Duration) {
	sr := c.schedule.SeriesOf(source)
	if sr != nil {
		c.state.Update(sr, source, position)
	}
}

// Next advances to the next segment after source at position.
// When the current series is exhausted it marks that series inactive and
// continues with the next active series in schedule order.
// Returns an error when source is unknown or no further segments exist.
func (c *Channel) Next(source Source, position time.Duration) error {
	seg, ok := c.schedule.NextSegmentAt(source, position, c.state.Shuffle, c.state.IsActive)
	if !ok {
		sr := c.schedule.SeriesOf(source)
		if sr == nil {
			return errors.New("no next segment")
		}
		c.state.Exhaust(sr.ID)
		seg, ok = c.schedule.firstActiveFrom(source, c.state.IsActive)
		if !ok {
			return errors.New("no next segment")
		}
	}
	sr := c.schedule.SeriesOf(seg.Source)
	if sr == nil {
		return errors.New("segment series not found")
	}
	c.state.Update(sr, seg.Source, seg.Clip.Start)
	return nil
}

// Jump sets playback to an arbitrary source and position, snapping to the
// nearest clip boundary. Returns an error if source is not in the schedule
// or position falls outside all clips.
func (c *Channel) Jump(source Source, position time.Duration) error {
	sr := c.schedule.SeriesOf(source)
	if sr == nil {
		return errors.New("source not found in schedule")
	}
	seg, ok := c.schedule.CurrentSegmentAt(source, position, c.state.Shuffle, c.state.IsActive)
	if !ok {
		return errors.New("invalid position for source")
	}
	c.state.Jump(sr.ID, seg.Source, seg.Clip.Start)
	return nil
}

// AllSeries returns every series in the schedule.
func (c *Channel) AllSeries() []*Series {
	return c.schedule.Series
}

// State returns the current playback state.
func (c *Channel) State() *State {
	return c.state
}

// Schedule returns the underlying schedule.
func (c *Channel) Schedule() *Schedule {
	return c.schedule
}

// SeriesStateOf returns the last-known source and position for the series that contains source.
// Returns zero values if source is not in the schedule or no state has been recorded.
func (c *Channel) SeriesStateOf(source Source) (Source, time.Duration) {
	sr := c.schedule.SeriesOf(source)
	if sr == nil {
		return Source{}, 0
	}
	return c.state.GetSeriesState(sr.ID)
}

// SetShuffle enables or disables inter-series shuffle mode.
func (c *Channel) SetShuffle(shuffle bool) {
	c.state.Shuffle = shuffle
}

// ToggleSeriesActive flips the active/inactive status of the named series.
// Inactive series are skipped by Next and excluded from shuffle selection.
func (c *Channel) ToggleSeriesActive(seriesID string) {
	if c.state.IsActive(seriesID) {
		c.state.SetInactive(seriesID)
	} else {
		c.state.SetActive(seriesID)
	}
}
