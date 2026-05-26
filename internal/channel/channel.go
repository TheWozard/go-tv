package channel

import (
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

var errNoNextSegment = errors.New("no next segment")

// Channel is the runtime orchestrator that pairs a Schedule with a playback State.
// It exposes the four core player operations — CurrentSegment, Progress, Next, Jump —
// and is the only entry point for mutating state. All public methods are safe for
// concurrent use.
//
// Channel contains only domain logic. Persistence (loading from disk, auto-save)
// is handled by store.ChannelStore.
type Channel struct {
	mu       sync.RWMutex
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	src, pos := c.state.Get()
	if src.IsZero() {
		seg, _ := c.schedule.First(c.state.IsActive)
		return seg
	}
	seg, ok := c.schedule.CurrentSegmentAt(src, pos)
	if !ok {
		seg, _ = c.schedule.First(c.state.IsActive)
	}
	return seg
}

// Progress records the current playback position without advancing.
// Called periodically by the player to persist where the viewer is within the episode.
func (c *Channel) Progress(source Source, position time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sr := c.schedule.SeriesOf(source)
	if sr != nil {
		c.state.SetPosition(sr.ID, source, position)
	}
}

// Next advances to the next segment after source at position.
// If another clip exists within the current episode it advances to that clip.
// Otherwise, in shuffle mode a random active series is chosen; in ordered mode
// the series advances sequentially, exhausting the current series when done.
// Returns an error when source is unknown or no further segments exist.
func (c *Channel) Next(source Source, position time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ep := c.schedule.FindEpisode(source)
	if ep == nil {
		return errors.New("unknown source")
	}
	series := c.schedule.SeriesOf(source)
	if clip, ok := ep.ClipAfter(position); ok {
		c.state.activateIfCurrent(series.ID, source, clip.Start)
		return nil
	}
	err := c.orderedNext(series.ID, source)
	if (c.state.Shuffle && ep.Mode != EpisodeContinueMode) || errors.Is(err, errNoNextSegment) {
		return c.shuffleActive()
	}
	return err
}

// shuffleActive picks a random active series and makes it active.
// If the picked series has no stored playback position, it is initialized
// to its first episode so that CurrentSegment never falls back to the
// schedule-order first episode of a different series.
func (c *Channel) shuffleActive() error {
	active := c.schedule.ActiveSeries(c.state.IsActive)
	if len(active) == 0 {
		return errors.New("no active series")
	}
	picked := active[rand.IntN(len(active))]
	src, pos := c.state.GetSeriesState(picked.ID)
	if src.IsZero() {
		if seg, ok := picked.FirstSegmentFrom(0, 0); ok {
			src, pos = seg.Source, seg.Clip.Start
		}
	}
	c.state.Activate(picked.ID, src, pos)
	return nil
}

// orderedNext advances to the next episode within the current series.
// Exhausts the series and returns errNoNextSegment when no further episodes exist.
func (c *Channel) orderedNext(id string, source Source) error {
	seg, ok := c.schedule.NextEpisodeInSeries(source)
	if !ok {
		c.state.Exhaust(id)
		return errNoNextSegment
	}
	c.state.activateIfCurrent(id, seg.Source, seg.Clip.Start)
	return nil
}

// NextEpisode advances to the next episode in the current series without shuffle.
// Returns an error if source is unknown or no further episode exists.
func (c *Channel) NextEpisode(source Source) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	series := c.schedule.SeriesOf(source)
	if series == nil {
		return errors.New("unknown source")
	}
	return c.orderedNext(series.ID, source)
}

// PrevEpisode moves to the previous episode within the current series.
// Returns an error if source is unknown or no earlier episode exists.
func (c *Channel) PrevEpisode(source Source) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	series := c.schedule.SeriesOf(source)
	if series == nil {
		return errors.New("unknown source")
	}
	seg, ok := c.schedule.PrevEpisodeInSeries(source)
	if !ok {
		return errors.New("no previous episode")
	}
	c.state.activateIfCurrent(series.ID, seg.Source, seg.Clip.Start)
	return nil
}

// Jump sets playback to an arbitrary source and position, snapping to the
// nearest clip boundary. Returns an error if source is not in the schedule
// or position falls outside all clips.
func (c *Channel) Jump(source Source, position time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sr := c.schedule.SeriesOf(source)
	if sr == nil {
		return errors.New("source not found in schedule")
	}
	seg, ok := c.schedule.CurrentSegmentAt(source, position)
	if !ok {
		return errors.New("invalid position for source")
	}
	c.state.Activate(sr.ID, seg.Source, position)
	return nil
}

// SetSeriesMode sets the playback mode for the series with the given ID.
func (c *Channel) SetSeriesMode(seriesID string, mode SeriesMode) error {
	for _, sr := range c.schedule.Series {
		if sr.ID == seriesID {
			sr.Mode = mode
			return nil
		}
	}
	return errors.New("series not found")
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	sr := c.schedule.SeriesOf(source)
	if sr == nil {
		return Source{}, 0
	}
	return c.state.GetSeriesState(sr.ID)
}

// SetShuffle enables or disables inter-series shuffle mode.
func (c *Channel) SetShuffle(shuffle bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Shuffle = shuffle
}

// ActivateSeries makes the named series the currently playing series.
// If the series was inactive it is also marked active. Falls back to the
// series's first segment if no playback position has been recorded yet.
func (c *Channel) ActivateSeries(seriesID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var sr *Series
	for _, s := range c.schedule.Series {
		if s.ID == seriesID {
			sr = s
			break
		}
	}
	if sr == nil {
		return errors.New("series not found")
	}
	c.state.SetActive(seriesID)
	src, pos := c.state.GetSeriesState(seriesID)
	if src.IsZero() {
		if seg, ok := sr.FirstSegmentFrom(0, 0); ok {
			src, pos = seg.Source, seg.Clip.Start
		}
	}
	c.state.Activate(seriesID, src, pos)
	return nil
}

// ToggleSeriesActive flips the active/inactive status of the named series.
// Inactive series are skipped by Next and excluded from shuffle selection.
// If the series being deactivated is the currently active series, a new active
// series is picked from the remaining active series.
func (c *Channel) ToggleSeriesActive(seriesID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.IsActive(seriesID) {
		c.state.SetInactive(seriesID)
		if c.state.ActiveSeries == seriesID {
			if err := c.shuffleActive(); err != nil {
				c.state.ActiveSeries = ""
			}
		}
	} else {
		c.state.SetActive(seriesID)
	}
}
