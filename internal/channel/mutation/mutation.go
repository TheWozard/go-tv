package mutation

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"go-tv/internal/channel"
)

// SeasonOrder describes the desired order of a season's episodes after reordering.
type SeasonOrder struct {
	Name       string
	EpisodeIDs []string
}

// CutRange is a time range to be cut from an episode during sponsorblock application.
type CutRange struct {
	Start time.Duration
	End   time.Duration
}

// RenameSeason renames a season within the named series (by series ID).
func RenameSeason(sc *channel.Schedule, seriesID, oldName, newName string) error {
	sr := findSeriesByID(sc, seriesID)
	if sr == nil {
		return errors.New("series not found")
	}
	for i, s := range sr.Seasons {
		if s.Name == oldName {
			sr.Seasons[i] = channel.NewSeason(newName, s.Episodes...)
			return nil
		}
	}
	return errors.New("season not found")
}

// ReorderSeries reorders the seasons and episodes of the named series (by series ID).
func ReorderSeries(sc *channel.Schedule, seriesID string, orders []SeasonOrder) error {
	sr := findSeriesByID(sc, seriesID)
	if sr == nil {
		return errors.New("series not found")
	}

	epByID := make(map[string]channel.Episode)
	for _, s := range sr.Seasons {
		for _, ep := range s.Episodes {
			epByID[ep.Source.ID] = ep
		}
	}

	newSeasons := make([]channel.Season, 0, len(orders))
	for _, order := range orders {
		eps := make([]channel.Episode, 0, len(order.EpisodeIDs))
		for _, id := range order.EpisodeIDs {
			if ep, ok := epByID[id]; ok {
				eps = append(eps, ep)
			}
		}
		newSeasons = append(newSeasons, channel.NewSeason(order.Name, eps...))
	}
	sr.Seasons = newSeasons
	sc.RebuildIndex()
	return nil
}

// SetEpisodeMode sets the EpisodeMode on the episode identified by source.
// Returns a pointer to the updated episode, or an error if not found.
func SetEpisodeMode(sc *channel.Schedule, source channel.Source, mode channel.EpisodeMode) (*channel.Episode, error) {
	ep := sc.FindEpisode(source)
	if ep == nil {
		return nil, errors.New("episode not found")
	}
	*ep = ep.WithMode(mode)
	return ep, nil
}

// ApplyCuts applies sponsorblock cut ranges to the episode identified by source.
// It returns a pointer to the updated episode.
func ApplyCuts(sc *channel.Schedule, source channel.Source, cuts []CutRange) (*channel.Episode, error) {
	ep := sc.FindEpisode(source)
	if ep == nil {
		return nil, errors.New("episode not found")
	}
	*ep = ep.WithClips(cutsToClips(ep.Length, cuts)...)
	return ep, nil
}

func cutsToClips(length time.Duration, cuts []CutRange) []channel.Clip {
	if len(cuts) == 0 {
		return nil
	}
	sorted := slices.Clone(cuts)
	slices.SortFunc(sorted, func(a, b CutRange) int {
		return cmp.Compare(a.Start, b.Start)
	})
	var clips []channel.Clip
	pos := time.Duration(0)
	for _, cut := range sorted {
		if pos < cut.Start {
			clips = append(clips, channel.NewClip(pos, cut.Start))
		}
		if cut.End > pos {
			pos = cut.End
		}
	}
	if pos < length {
		clips = append(clips, channel.NewClip(pos, length))
	}
	return clips
}


func findSeriesByID(sc *channel.Schedule, id string) *channel.Series {
	for _, sr := range sc.Series {
		if sr.ID == id {
			return sr
		}
	}
	return nil
}
