package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type persisted struct {
	VideoID  string `json:"video_id"`
	Position string `json:"position"` // e.g. "1h2m3s"
}

type State struct {
	mu      sync.RWMutex
	VideoID string
	Seconds float64
}

func New(videoID string) *State {
	return &State{VideoID: videoID}
}

func Load(path string, def *State) *State {
	f, err := os.Open(path)
	if err != nil {
		return def
	}
	defer f.Close()
	var p persisted
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		return def
	}
	d, err := time.ParseDuration(p.Position)
	if err != nil {
		return def
	}
	return &State{
		VideoID: p.VideoID,
		Seconds: d.Seconds(),
	}
}

func (s *State) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := time.Duration(s.Seconds * float64(time.Second)).Truncate(time.Second)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(persisted{
		VideoID:  s.VideoID,
		Position: d.String(),
	})
}

func (s *State) Get() (videoID string, seconds float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.VideoID, s.Seconds
}

func (s *State) SetPosition(videoID string, seconds float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.VideoID != videoID {
		return
	}
	s.Seconds = seconds
}

func (s *State) Jump(videoID string, seconds float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VideoID = videoID
	s.Seconds = seconds
}

func (s *State) Advance(currentVideoID, nextVideoID string, startSeconds float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.VideoID != currentVideoID {
		return
	}
	s.VideoID = nextVideoID
	s.Seconds = startSeconds
}

func (s *State) String() string {
	d := time.Duration(s.Seconds * float64(time.Second)).Truncate(time.Second)
	return fmt.Sprintf("%s at %s", s.VideoID, d)
}
