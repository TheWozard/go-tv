package channel

import "time"

// State tracks which series is currently playing and where playback is within it.
// It also records which series have been fully exhausted (inactive).
//
// State is keyed by Series.ID. All series are active by default;
// a series becomes inactive when Channel.Next exhausts it, and can be reactivated
// via ToggleSeriesActive.
type State struct {
	// ActiveSeries is the ID of the series currently being played.
	ActiveSeries string
	// Shuffle enables inter-series shuffle mode. When true, after each episode
	// ends playback jumps to a random active series.
	Shuffle  bool
	series   map[string]SeriesState
	inactive map[string]bool
}

// SeriesState holds the last-known playback position for a single series.
type SeriesState struct {
	Source   Source
	Position time.Duration
}

// NewEmptyState constructs a State with no playback history and all series active.
func NewEmptyState() *State {
	return &State{series: make(map[string]SeriesState), inactive: make(map[string]bool)}
}

// SetInactive marks a series as inactive without clearing its playback position.
// Use for user-initiated deactivation where the position should be preserved.
func (s *State) SetInactive(id string) {
	if s.inactive == nil {
		s.inactive = make(map[string]bool)
	}
	s.inactive[id] = true
}

// Exhaust marks a series inactive and clears its stored playback position.
// Use when a series is exhausted by playback reaching the end.
func (s *State) Exhaust(id string) {
	s.SetInactive(id)
	delete(s.series, id)
}

// SetActive removes a series from the inactive set, making it eligible for playback again.
func (s *State) SetActive(id string) {
	delete(s.inactive, id)
}

// IsActive reports whether a series is eligible for playback.
// Series are active by default; only explicitly finished ones are inactive.
func (s *State) IsActive(id string) bool {
	return !s.inactive[id]
}

// EachInactiveSeries calls fn for every series marked inactive.
func (s *State) EachInactiveSeries(fn func(id string)) {
	for id := range s.inactive {
		fn(id)
	}
}

// NewStateFor constructs a State with a single series already positioned at src/pos.
// Used by the store when bootstrapping from a saved state file.
func NewStateFor(seriesID string, src Source, pos time.Duration) *State {
	s := NewEmptyState()
	s.Activate(seriesID, src, pos)
	return s
}

// Get returns the source and position of the active series.
func (s *State) Get() (Source, time.Duration) {
	if st, ok := s.series[s.ActiveSeries]; ok {
		return st.Source, st.Position
	}
	return Source{}, 0
}

// GetSeriesState returns the source and position for the given series ID.
func (s *State) GetSeriesState(id string) (Source, time.Duration) {
	if st, ok := s.series[id]; ok {
		return st.Source, st.Position
	}
	return Source{}, 0
}

// SetSeriesState sets the playback state for a series without changing ActiveSeries.
func (s *State) SetSeriesState(id string, src Source, pos time.Duration) {
	s.series[id] = SeriesState{Source: src, Position: pos}
}

// SetPosition records the current playback position for a series without
// changing which series is active. Use from Progress to avoid stale updates
// overriding an in-flight Jump or Next.
func (s *State) SetPosition(seriesID string, src Source, pos time.Duration) {
	s.series[seriesID] = SeriesState{Source: src, Position: pos}
}

// Activate sets the active series and records its current playback position.
// Use from Next and Jump when the playing series intentionally changes.
func (s *State) Activate(seriesID string, src Source, pos time.Duration) {
	s.ActiveSeries = seriesID
	s.series[seriesID] = SeriesState{Source: src, Position: pos}
}

// EachSeriesState calls fn for every series with persisted state.
func (s *State) EachSeriesState(fn func(id string, src Source, pos time.Duration)) {
	for id, st := range s.series {
		fn(id, st.Source, st.Position)
	}
}
