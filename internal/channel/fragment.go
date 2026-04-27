package channel

import "time"

// Fragment describes the next piece of a source to play.
type Fragment struct {
	Source Source
	Start  time.Duration
	End    time.Duration
}

// toState converts this fragment into a persisted state snapshot.
func (f Fragment) toState(path string) *State {
	return &State{
		path:     path,
		ID:       f.Source,
		Position: Duration{f.Start},
	}
}
