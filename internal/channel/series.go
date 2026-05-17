package channel

import (
	"iter"
	"regexp"
	"strings"
)

type SeriesMode string

const (
	LoopMode SeriesMode = "loop"
	Single   SeriesMode = "single"
)

type Series struct {
	Name    string
	Mode    SeriesMode
	Seasons []Season
}

func NewSeries(mode SeriesMode, seasons ...Season) Series {
	return Series{Mode: mode, Seasons: seasons}
}

var reSpecial = regexp.MustCompile(`[^a-z0-9]+`)

func (sr Series) ID() string {
	s := strings.ToLower(sr.Name)
	s = reSpecial.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// IterSegmentsFrom yields all segments starting from (seasonIdx, episodeIdx).
// In LoopMode the remaining seasons are followed by all earlier seasons.
func (sr Series) IterSegmentsFrom(seasonIdx, episodeIdx int) iter.Seq[Segment] {
	return func(yield func(Segment) bool) {
		start := min(seasonIdx, len(sr.Seasons))
		seasons := sr.Seasons[start:]
		if sr.Mode == LoopMode {
			seasons = append(seasons, sr.Seasons[:start]...)
		}
		for _, s := range seasons {
			for seg := range s.IterSegmentsFrom(episodeIdx) {
				if !yield(seg) {
					return
				}
			}
			episodeIdx = 0
		}
	}
}
