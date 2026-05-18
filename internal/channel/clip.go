package channel

import (
	"cmp"
	"time"
)

// Clip is a half-open time range [Start, End) within a video.
// When an Episode has no explicit clips, a synthetic clip spanning [0, Length) is used.
type Clip struct {
	Start time.Duration
	End   time.Duration
}

// NewClip constructs a Clip. If start > end, start is clamped to end.
func NewClip(start, end time.Duration) Clip {
	if start > end {
		start = end
	}
	return Clip{Start: start, End: end}
}

// Compare orders clips by Start time. Used for sorting.
func (c Clip) Compare(o Clip) int {
	return cmp.Compare(c.Start, o.Start)
}
