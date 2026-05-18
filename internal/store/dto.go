package store

import (
	"time"

	"go-tv/internal/channel"
)

// sourceDTO is the JSON representation of a channel.Source.
type sourceDTO struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// segmentDTO is the JSON representation of a channel.Clip.
type segmentDTO struct {
	Start Duration `json:"start,omitzero"`
	End   Duration `json:"end,omitzero"`
}

// episodeDTO is the JSON representation of a channel.Episode.
type episodeDTO struct {
	Source   sourceDTO    `json:"source"`
	Title    string       `json:"title,omitempty"`
	Segments []segmentDTO `json:"segments,omitempty"`
	Length   Duration     `json:"length"`
	Continue bool         `json:"continue,omitempty"`
}

// seasonDTO is the JSON representation of a channel.Season.
type seasonDTO struct {
	Name     string       `json:"name"`
	Episodes []episodeDTO `json:"episodes"`
}

// seriesDTO is the JSON representation of a channel.Series.
type seriesDTO struct {
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name"`
	Mode    string      `json:"mode,omitempty"`
	Seasons []seasonDTO `json:"seasons"`
}

// seriesStateDTO is the JSON representation of per-series playback state.
type seriesStateDTO struct {
	Source   sourceDTO `json:"source"`
	Position Duration  `json:"position"`
}

// stateDTO is the JSON representation of channel.State.
type stateDTO struct {
	ActiveSeries   string                     `json:"active_series,omitempty"`
	Shuffle        bool                       `json:"shuffle,omitempty"`
	Series         map[string]*seriesStateDTO `json:"series,omitempty"`
	InactiveSeries []string                   `json:"inactive_series,omitempty"`
}

// toSeriesDTO converts a channel.Series to its JSON DTO.
func toSeriesDTO(s *channel.Series) seriesDTO {
	dto := seriesDTO{
		ID:      s.ID,
		Name:    s.Name,
		Mode:    string(s.Mode),
		Seasons: make([]seasonDTO, len(s.Seasons)),
	}
	for i, season := range s.Seasons {
		eps := make([]episodeDTO, len(season.Episodes))
		for j, ep := range season.Episodes {
			eDTO := episodeDTO{
				Source:   sourceDTO{Kind: string(ep.Source.Kind), ID: ep.Source.ID},
				Title:    ep.Title,
				Length:   Duration{ep.Length},
				Continue: ep.Mode == channel.EpisodeContinueMode,
			}
			if len(ep.Clips) > 0 {
				eDTO.Segments = make([]segmentDTO, len(ep.Clips))
				for k, clip := range ep.Clips {
					eDTO.Segments[k] = segmentDTO{
						Start: Duration{clip.Start},
						End:   Duration{clip.End},
					}
				}
			}
			eps[j] = eDTO
		}
		dto.Seasons[i] = seasonDTO{Name: season.Name, Episodes: eps}
	}
	return dto
}

// fromSeriesDTO converts a seriesDTO back to a channel.Series.
// If the persisted ID is missing (legacy data), a new one is generated.
func fromSeriesDTO(dto seriesDTO) *channel.Series {
	id := dto.ID
	if id == "" {
		id = channel.NewSeriesID()
	}
	mode := channel.SeriesMode(dto.Mode)
	seasons := make([]channel.Season, len(dto.Seasons))
	for i, sDTO := range dto.Seasons {
		eps := make([]channel.Episode, len(sDTO.Episodes))
		for j, eDTO := range sDTO.Episodes {
			src, _ := channel.NewValidatedSource(channel.SourceKind(eDTO.Source.Kind), eDTO.Source.ID)
			clips := make([]channel.Clip, len(eDTO.Segments))
			for k, seg := range eDTO.Segments {
				start := seg.Start.Duration
				end := seg.End.Duration
				if end == 0 {
					end = eDTO.Length.Duration
				}
				clips[k] = channel.NewClip(start, end)
			}
			ep := CleanEpisode(channel.NewEpisode(src, eDTO.Length.Duration, clips...).WithTitle(eDTO.Title))
			if eDTO.Continue {
				ep = ep.WithMode(channel.EpisodeContinueMode)
			}
			eps[j] = ep
		}
		seasons[i] = channel.NewSeason(sDTO.Name, eps...)
	}
	return channel.NewSeriesWithID(id, dto.Name, mode, seasons...)
}

// toStateDTO converts a channel.State to its JSON DTO.
func toStateDTO(s *channel.State) stateDTO {
	dto := stateDTO{ActiveSeries: s.ActiveSeries, Shuffle: s.Shuffle}
	s.EachSeriesState(func(id string, src channel.Source, pos time.Duration) {
		if dto.Series == nil {
			dto.Series = make(map[string]*seriesStateDTO)
		}
		dto.Series[id] = &seriesStateDTO{
			Source:   sourceDTO{Kind: string(src.Kind), ID: src.ID},
			Position: Duration{pos},
		}
	})
	s.EachInactiveSeries(func(id string) {
		dto.InactiveSeries = append(dto.InactiveSeries, id)
	})
	return dto
}

// fromStateDTO converts a stateDTO back to a channel.State.
func fromStateDTO(dto stateDTO) *channel.State {
	state := channel.NewEmptyState()
	state.ActiveSeries = dto.ActiveSeries
	state.Shuffle = dto.Shuffle
	for id, ss := range dto.Series {
		src, _ := channel.NewValidatedSource(channel.SourceKind(ss.Source.Kind), ss.Source.ID)
		state.SetSeriesState(id, src, ss.Position.Duration)
	}
	for _, id := range dto.InactiveSeries {
		state.SetInactive(id)
	}
	return state
}
