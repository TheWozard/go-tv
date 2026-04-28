package channel

import "time"

// Video is a single playable entry.
type Video struct {
	Source   Source    `json:"source"`
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
			end := seg.EndDuration(v.Length.Duration)
			if end >= position {
				return Fragment{
					Source: v.Source,
					Start:  seg.StartDuration(),
					End:    end,
				}, true
			}
		}
	} else if position < v.Length.Duration {
		return Fragment{
			Source: v.Source,
			Start:  0,
			End:    v.Length.Duration,
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
			if seg.StartDuration() >= position {
				return Fragment{
					Source: v.Source,
					Start:  seg.StartDuration(),
					End:    seg.EndDuration(v.Length.Duration),
				}, true
			}
		}
	} else if position == 0 {
		return Fragment{
			Source: v.Source,
			Start:  0,
			End:    v.Length.Duration,
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

// EndDuration returns the segment's end, defaulting to def (typically the video length).
func (seg Segment) EndDuration(def time.Duration) time.Duration {
	if seg.End == nil {
		return def
	}
	return seg.End.Duration
}
