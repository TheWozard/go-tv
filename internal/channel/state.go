package channel

import "time"

type State struct {
	Current string
	Series  map[string]SeriesState
}

type SeriesState struct {
	Source   Source
	Position time.Duration
}

func (s State) Get(series Series) (SeriesState, bool) {
	if state, ok := s.Series[series.ID()]; ok {
		return state, true
	}
	return SeriesState{}, false
}

func (s State) Update(series Series, source Source, position time.Duration) {
	id := series.ID()
	s.Current = id
	s.Series[id] = SeriesState{Source: source, Position: position}
}
