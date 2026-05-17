package channel

import (
	"iter"
	"slices"
	"time"
)

type EpisodeMode string

const (
	ShuffleEpisodeMode  EpisodeMode = "shuffle"
	ContinueEpisodeMode EpisodeMode = "continue"
)

type Episode struct {
	source Source
	title  string
	clips  []Clip
	length time.Duration
	mode   EpisodeMode
}

func NewEpisode(source Source, length time.Duration, clips ...Clip) Episode {
	return Episode{
		source: source,
		title:  "Unknown",
		length: length,
	}.WithClips(clips...)
}

func (e Episode) WithTitle(title string) Episode {
	e.title = title
	return e
}

func (e Episode) WithClips(clips ...Clip) Episode {
	e.clips = clips
	slices.SortFunc(e.clips, Clip.Compare)
	return e
}

func (e Episode) WithMode(mode EpisodeMode) Episode {
	e.mode = mode
	return e
}

func (e Episode) Clips() []Clip {
	clips := make([]Clip, 0, max(1, len(e.clips)))
	for c := range e.IterClips() {
		clips = append(clips, c)
	}
	return clips
}

func (e Episode) IterClips() iter.Seq[Clip] {
	return func(yield func(Clip) bool) {
		if len(e.clips) > 0 {
			for _, clip := range e.clips {
				if !yield(clip) {
					return
				}
			}
			return
		}
		yield(NewClip(0, e.length))
	}
}
