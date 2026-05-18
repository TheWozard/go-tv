package channel

import (
	"iter"
	"slices"
	"time"
)

// EpisodeMode controls per-episode playback behavior after the episode ends.
type EpisodeMode string

const (
	// EpisodeInheritMode is the default mode. The episode follows its series'
	// playback mode with no per-episode override.
	EpisodeInheritMode EpisodeMode = ""
	// EpisodeContinueMode overrides the series mode: when the episode ends,
	// playback always advances to the next episode in the series rather than
	// following the series-level loop or shuffle behavior.
	EpisodeContinueMode EpisodeMode = "continue"
)

// Episode is a single video within a Season. It carries a Source (the video to play),
// an optional Title, the total Length, and an optional list of Clips.
//
// Clips define sub-ranges of the video to include in playback (e.g. after SponsorBlock
// cuts have been applied). When Clips is empty, the full video [0, Length) is played.
// Clips are always stored sorted by Start time.
type Episode struct {
	Source Source
	Title  string
	Length time.Duration
	Mode   EpisodeMode
	// Clips are the sub-ranges to play. Empty means play the full video.
	// Use for UI visualization. Use IterClips / FirstClip / ClipAt for playback.
	Clips []Clip
}

// NewEpisode constructs an Episode with the given source, length, and optional clips.
// Clips are sorted by start time. Title defaults to "Unknown".
func NewEpisode(source Source, length time.Duration, clips ...Clip) Episode {
	return Episode{
		Source: source,
		Title:  "Unknown",
		Length: length,
	}.WithClips(clips...)
}

// WithTitle returns a copy of the episode with the given title.
func (e Episode) WithTitle(title string) Episode {
	e.Title = title
	return e
}

// WithClips returns a copy of the episode with the given clips, sorted by start time.
func (e Episode) WithClips(clips ...Clip) Episode {
	e.Clips = clips
	slices.SortFunc(e.Clips, Clip.Compare)
	return e
}

// WithMode returns a copy of the episode with the given mode.
func (e Episode) WithMode(mode EpisodeMode) Episode {
	e.Mode = mode
	return e
}

// IterClips iterates the playback clips. If no explicit clips are set, a single
// synthetic clip spanning [0, Length) is yielded. Use for all-clips iteration;
// use FirstClip / ClipAt / ClipAfter for single-result lookups.
func (e Episode) IterClips() iter.Seq[Clip] {
	return func(yield func(Clip) bool) {
		if len(e.Clips) > 0 {
			for _, clip := range e.Clips {
				if !yield(clip) {
					return
				}
			}
			return
		}
		yield(NewClip(0, e.Length))
	}
}

// FirstClip returns the first playback clip. If no explicit clips are set and
// Length > 0, a synthetic clip spanning [0, Length) is returned.
func (e Episode) FirstClip() (Clip, bool) {
	if len(e.Clips) > 0 {
		return e.Clips[0], true
	}
	if e.Length > 0 {
		return NewClip(0, e.Length), true
	}
	return Clip{}, false
}

// ClipAt returns the clip that contains pos, i.e. the first clip where pos < clip.End.
// Returns false when pos is past all clips (the episode is finished).
func (e Episode) ClipAt(pos time.Duration) (Clip, bool) {
	for clip := range e.IterClips() {
		if pos < clip.End {
			return clip, true
		}
	}
	return Clip{}, false
}

// ClipAfter returns the first clip whose Start is strictly after pos.
// Used to advance within an episode when a clip boundary is reached.
func (e Episode) ClipAfter(pos time.Duration) (Clip, bool) {
	for clip := range e.IterClips() {
		if clip.Start > pos {
			return clip, true
		}
	}
	return Clip{}, false
}
