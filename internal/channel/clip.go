package channel

import (
	"cmp"
	"time"
)

type Clip struct {
	start time.Duration
	end   time.Duration
}

func NewClip(start, end time.Duration) Clip {
	return Clip{start: start, end: end}
}

func (c Clip) Start() time.Duration {
	return c.start
}

func (c Clip) End() time.Duration {
	return c.end
}

func (c Clip) Window() (time.Duration, time.Duration) {
	return c.start, c.end
}

func (c Clip) Compare(o Clip) int {
	return cmp.Compare(c.start, o.start)
}

func (c Clip) At(position time.Duration) Clip {
	return NewClip(min(max(c.start, position), c.end), c.end)
}
