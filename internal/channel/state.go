package channel

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

func NewState(source Source, position time.Duration) *State {
	return &State{
		Source:   source,
		Position: Duration{position},
	}
}

// State tracks the current playback position within the channel.
// It is persisted to disk as JSON so playback survives restarts.
type State struct {
	mu   sync.RWMutex
	path string

	Source   Source   `json:"source"`
	Position Duration `json:"position"`
}

// LoadState reads persisted state from path. If the file is missing, corrupt,
// or refers to a video/position no longer valid in the schedule, it falls back
// to the first playable fragment.
func LoadState(path string, schedule *Schedule) *State {
	first, _ := schedule.First()
	f, err := os.Open(path)
	if err != nil {
		return first.toState(path)
	}
	defer f.Close()

	var state State
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return first.toState(path)
	}
	if current, ok := schedule.Current(state.Source, state.Position.Duration); ok {
		if !current.Source.Equal(state.Source) || current.Start > state.Position.Duration {
			return current.toState(path)
		}
		state.path = path
		return &state
	}
	return first.toState(path)
}

// Save persists the current state to disk.
func (s *State) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s)
}

// Get returns the current source and playback position.
func (s *State) Get() (Source, time.Duration) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Source, s.Position.Duration
}

// SetPosition updates the playback position for the current video.
// The update is ignored if id does not match the current source (stale report).
func (s *State) SetPosition(source Source, position time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Source.Equal(source) {
		return
	}
	s.Position.Duration = position.Round(time.Second)
}

// Jump unconditionally moves playback to a new source and position.
func (s *State) Jump(source Source, position time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Source = source
	s.Position.Duration = position.Round(time.Second)
}

// Advance transitions to the next fragment. The transition is ignored if
// currentSource does not match the active source (another advance already occurred).
func (s *State) Advance(currentSource, nextSource Source, position time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Source.Equal(currentSource) {
		return
	}
	s.Source = nextSource
	s.Position.Duration = position
}

func (s *State) SetFilePath(path string) {
	s.path = path
}
