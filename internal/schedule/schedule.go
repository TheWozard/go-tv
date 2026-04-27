package schedule

import (
	"encoding/json"
	"iter"
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

// Segment defines a playback window within a video.
// Start/End are pointers so zero values (play from start / play to end) are omitted.
type Segment struct {
	Start *Duration `json:"start,omitempty"`
	End   *Duration `json:"end,omitempty"`
}

// StartDuration returns the segment's start, defaulting to 0.
func (seg Segment) StartDuration() time.Duration {
	if seg.Start == nil {
		return 0
	}
	return seg.Start.Duration
}

func (seg Segment) EndDuration(def time.Duration) time.Duration {
	if seg.End == nil {
		return def
	}
	return seg.End.Duration
}

// Video is a single playable entry.
type Video struct {
	ID       string    `json:"id"`
	Title    string    `json:"title,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
	Length   Duration  `json:"length"`
}

// Current returns the [Fragment] of the video that the position is a part of.
// If the position is between [Fragments], the next Fragment is returned.
// Returns false when position is after all [Fragments]
func (v Video) Current(position time.Duration) (Fragment, bool) {
	if len(v.Segments) > 0 {
		for _, seg := range v.Segments {
			if seg.StartDuration() < position {
				return Fragment{
					ID:    v.ID,
					Start: seg.StartDuration(),
					End:   seg.EndDuration(v.Length.Duration),
				}, true
			}
		}
	} else if position < v.Length.Duration {
		return Fragment{
			ID:    v.ID,
			Start: 0,
			End:   v.Length.Duration,
		}, true
	}
	return Fragment{}, false
}

// Next returns the [Fragment] of the video that is next after the position.
// If the position is currently inside of a [Fragment], the next [Fragment] is returned.
// Returns false when position is after the start of the last [Fragment] even if it is
// before the end.
func (v Video) Next(position time.Duration) (Fragment, bool) {
	if len(v.Segments) > 0 {
		for _, seg := range v.Segments {
			if seg.StartDuration() > position {
				return Fragment{
					ID:    v.ID,
					Start: seg.StartDuration(),
					End:   seg.EndDuration(v.Length.Duration),
				}, true
			}
		}
	} else if position == 0 {
		return Fragment{
			ID:    v.ID,
			Start: 0,
			End:   v.Length.Duration,
		}, true
	}
	return Fragment{}, false
}

// Clean removes redundant start=0 and end=length values from segments,
// then drops any segments that become empty. Modifies the video in place.
func (v *Video) Clean() {
	var cleaned []Segment
	for _, s := range v.Segments {
		if s.Start != nil && s.Start.Duration == 0 {
			s.Start = nil
		}
		if s.End != nil && s.End.Duration == v.Length.Duration {
			s.End = nil
		}
		if s.Start != nil || s.End != nil {
			cleaned = append(cleaned, s)
		}
	}
	v.Segments = cleaned
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

// iter returns an iterator over all videos in playback order.
// Must be called with lock held.
func (s *Schedule) iter() iter.Seq[Video] {
	return func(yield func(Video) bool) {
		for _, item := range s.Items {
			for _, v := range item.Videos {
				if !yield(v) {
					return
				}
			}
		}
	}
}

func (s *Schedule) Find(videoID string) (*Video, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for v := range s.iter() {
		if v.ID == videoID {
			return &v, true
		}
	}
	return nil, false
}

// Fragment describes the next piece of video to play.
type Fragment struct {
	ID    string
	Start time.Duration
	End   time.Duration
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

func (s *Schedule) Current(videoID string, position time.Duration) (Fragment, bool) {
	return s.findFragment(videoID, position, Video.Current)
}

func (s *Schedule) Next(videoID string, position time.Duration) (Fragment, bool) {
	return s.findFragment(videoID, position, Video.Next)
}

func (s *Schedule) findFragment(videoID string, position time.Duration, check func(Video, time.Duration) (Fragment, bool)) (Fragment, bool) {
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

		if v.ID != videoID {
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
