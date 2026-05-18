package store

import "go-tv/internal/channel"

// CleanEpisode removes zero-length clips and a redundant full-video clip
// (a single clip spanning 0→Length is redundant because Clips() synthesises
// one as a fallback when no explicit clips exist).
func CleanEpisode(ep channel.Episode) channel.Episode {
	clips := ep.Clips
	filtered := make([]channel.Clip, 0, len(clips))
	for _, c := range clips {
		if c.Start == c.End {
			continue
		}
		if c.Start == 0 && c.End == ep.Length {
			continue
		}
		filtered = append(filtered, c)
	}
	return ep.WithClips(filtered...)
}
