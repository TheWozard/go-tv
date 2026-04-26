package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Duration wraps time.Duration to support JSON marshal/unmarshal as a Go
// duration string (e.g. "3m33s").
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

type Video struct {
	ID     string    `json:"id"`
	Title  string    `json:"title,omitempty"`
	Start  *Duration `json:"start,omitempty"` // nil means start from 0
	Stop   Duration  `json:"stop"`
	Length Duration  `json:"length"` // total video duration; used as slider max
}

// StartSeconds returns the start offset in seconds, defaulting to 0.
func (v *Video) StartSeconds() float64 {
	if v.Start == nil {
		return 0
	}
	return v.Start.Seconds()
}

type Schedule struct {
	mu     sync.RWMutex
	Videos []Video `json:"videos"`
}

func Load(path string) (*Schedule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s Schedule
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Schedule) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// All returns a shallow copy of the video slice, safe for reading concurrently.
func (s *Schedule) All() []Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Video, len(s.Videos))
	copy(out, s.Videos)
	return out
}

// Update replaces the video list atomically.
func (s *Schedule) Update(videos []Video) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Videos = videos
}

func (s *Schedule) First() *Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Videos) == 0 {
		return nil
	}
	return &s.Videos[0]
}

func (s *Schedule) Find(videoID string) (*Video, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.Videos {
		if s.Videos[i].ID == videoID {
			return &s.Videos[i], true
		}
	}
	return nil, false
}

func (s *Schedule) Next(videoID string) (*Video, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, v := range s.Videos {
		if v.ID == videoID {
			return &s.Videos[(i+1)%len(s.Videos)], nil
		}
	}
	return nil, fmt.Errorf("video %q not found in schedule", videoID)
}
