package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type persisted struct {
	VideoID  string `json:"video_id"`
	Position string `json:"position"` // e.g. "1h2m3.4s"
}

func Load(path string) (*State, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var p persisted
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		return nil, err
	}
	d, err := time.ParseDuration(p.Position)
	if err != nil {
		return nil, err
	}
	return &State{
		VideoID:   p.VideoID,
		StartedAt: time.Now().Add(-d),
	}, nil
}

func (s *State) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(persisted{
		VideoID:  s.VideoID,
		Position: time.Since(s.StartedAt).Truncate(time.Second).String(),
	})
}

type State struct {
	mu        sync.RWMutex
	VideoID   string
	StartedAt time.Time
}

func New(videoID string) *State {
	return &State{
		VideoID:   videoID,
		StartedAt: time.Now(),
	}
}

func (s *State) Get() (videoID string, startedAt time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.VideoID, s.StartedAt
}

// Advance moves to nextVideoID only if we're still on currentVideoID.
// First caller wins; subsequent calls with the same currentVideoID are no-ops.
func (s *State) Advance(currentVideoID, nextVideoID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.VideoID != currentVideoID {
		return
	}
	s.VideoID = nextVideoID
	s.StartedAt = time.Now()
}
