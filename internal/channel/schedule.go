package channel

import (
	"encoding/json"
	"iter"
	"os"
	"sync"
	"time"
)

// Playlist is a named group of one or more videos.
// Single-video imports produce a Playlist with one entry in Videos.
type Playlist struct {
	Name   string  `json:"name"`
	Videos []Video `json:"videos"`
}

// Schedule is the ordered playlist of items that defines a channel's content.
// It is persisted to disk as JSON and all methods are safe for concurrent use.
type Schedule struct {
	mu   sync.RWMutex
	path string

	Playlists []Playlist `json:"playlists"`
}

// NewSchedule creates an empty schedule that will persist to path.
func NewSchedule(path string) *Schedule {
	return &Schedule{path: path}
}

// LoadSchedule reads and decodes a schedule from the given JSON file.
func LoadSchedule(path string) (*Schedule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s Schedule
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	s.path = path
	return &s, nil
}

// Save writes the schedule to disk as pretty-printed JSON.
func (s *Schedule) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// iter returns an iterator over all videos in playback order.
// Must be called with lock held.
func (s *Schedule) iter() iter.Seq[Video] {
	return func(yield func(Video) bool) {
		for _, item := range s.Playlists {
			for _, v := range item.Videos {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Find locates the first video matching the given source.
func (s *Schedule) Find(id Source) (*Video, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for v := range s.iter() {
		if v.Source.Equal(id) {
			return &v, true
		}
	}
	return nil, false
}

// First returns the first possible [Fragment] for the current schedule
func (s *Schedule) First() (Fragment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for v := range s.iter() {
		if f, ok := v.Next(0); ok {
			return f, true
		}
	}
	return Fragment{}, false
}

// Current returns the fragment containing the given position within the
// identified video. If the position falls in a gap between segments, the next
// segment's fragment is returned.
func (s *Schedule) Current(id Source, position time.Duration) (Fragment, bool) {
	return s.findFragment(id, position, Video.Current)
}

// Next returns the fragment that follows the given position. If the current
// video has no further segments, it moves to the next video in the schedule.
// Wraps to the beginning when the end is reached.
func (s *Schedule) Next(id Source, position time.Duration) (Fragment, bool) {
	return s.findFragment(id, position, Video.Next)
}

func (s *Schedule) findFragment(source Source, position time.Duration, check func(Video, time.Duration) (Fragment, bool)) (Fragment, bool) {
	if source.Equal(Source{}) {
		return s.First()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	found := false
	for v := range s.iter() {
		if found {
			if f, ok := check(v, 0); ok {
				return f, true
			}
			continue
		}

		if !v.Source.Equal(source) {
			continue
		}
		if f, ok := check(v, position); ok {
			return f, true
		}
		found = true
	}
	return s.First()
}

// All returns the flat playback order, safe for concurrent use.
func (s *Schedule) All() []Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Video
	for v := range s.iter() {
		out = append(out, v)
	}
	return out
}

// AllItems returns a shallow copy of the items slice, safe for concurrent use.
func (s *Schedule) AllItems() []Playlist {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Playlist, len(s.Playlists))
	copy(out, s.Playlists)
	return out
}

// Update replaces the item list atomically.
func (s *Schedule) Update(items []Playlist) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Playlists = items
}
