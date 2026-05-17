package channel

type Segment struct {
	source Source
	clip   Clip
}

func NewSegment(source Source, clip Clip) Segment {
	return Segment{source: source, clip: clip}
}

func (s Segment) Source() Source {
	return s.source
}

func (s Segment) Clip() Clip {
	return s.clip
}
