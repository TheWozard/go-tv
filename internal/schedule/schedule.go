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

// Video is a single playable entry.
type Video struct {
	ID     string    `json:"id"`
	Title  string    `json:"title,omitempty"`
	Start  *Duration `json:"start,omitempty"`
	Stop   Duration  `json:"stop"`
	Length Duration  `json:"length"`
}

func (v *Video) StartSeconds() float64 {
	if v.Start == nil {
		return 0
	}
	return v.Start.Seconds()
}

// Item is a named group of one or more videos.
// Single-video imports produce an Item with one entry in Videos.
type Item struct {
	Name   string  `json:"name"`
	Videos []Video `json:"videos"`
}

type Schedule struct {
	mu    sync.RWMutex
	Items []Item `json:"items"`
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

// flat returns all videos in playback order. Must be called with lock held.
func (s *Schedule) flat() []Video {
	var out []Video
	for _, item := range s.Items {
		out = append(out, item.Videos...)
	}
	return out
}

// All returns the flat playback order, safe for concurrent use.
func (s *Schedule) All() []Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flat()
}

// AllItems returns a shallow copy of the items slice, safe for concurrent use.
func (s *Schedule) AllItems() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, len(s.Items))
	copy(out, s.Items)
	return out
}

// Update replaces the item list atomically.
func (s *Schedule) Update(items []Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Items = items
}

func (s *Schedule) First() *Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vs := s.flat()
	if len(vs) == 0 {
		return nil
	}
	v := vs[0]
	return &v
}

func (s *Schedule) Find(videoID string) (*Video, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.flat() {
		if v.ID == videoID {
			vCopy := v
			return &vCopy, true
		}
	}
	return nil, false
}

func (s *Schedule) Next(videoID string) (*Video, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vs := s.flat()
	for i, v := range vs {
		if v.ID == videoID {
			next := vs[(i+1)%len(vs)]
			return &next, nil
		}
	}
	return nil, fmt.Errorf("video %q not found in schedule", videoID)
}
