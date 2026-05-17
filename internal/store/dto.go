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

// segmentDTO is the JSON representation of a channel.Segment.
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
	Name    string      `json:"name"`
	Seasons []seasonDTO `json:"seasons"`
}

// seriesStateDTO is the JSON representation of per-series playback state.
type seriesStateDTO struct {
	Source   sourceDTO `json:"source"`
	Position Duration  `json:"position"`
}

// stateDTO is the JSON representation of channel.State.
type stateDTO struct {
	ActiveSeries string                     `json:"active_series,omitempty"`
	Series       map[string]*seriesStateDTO `json:"series,omitempty"`
}

// toSeriesDTO converts a channel.Series to its JSON DTO.
func toSeriesDTO(s *channel.Series) seriesDTO {
	seasons := s.AllSeasons()
	dto := seriesDTO{Name: s.Name, Seasons: make([]seasonDTO, len(seasons))}
	for i, season := range seasons {
		eps := make([]episodeDTO, len(season.Episodes))
		for j, ep := range season.Episodes {
			eDTO := episodeDTO{
				Source:   sourceDTO{Kind: string(ep.Source.Kind), ID: ep.Source.ID},
				Title:    ep.Title,
				Length:   Duration{ep.Length},
				Continue: ep.Continue,
			}
			if len(ep.Clips) > 0 {
				eDTO.Segments = make([]segmentDTO, len(ep.Clips))
				for k, seg := range ep.Clips {
					eDTO.Segments[k] = segmentDTO{
						Start: Duration{seg.Start},
						End:   Duration{seg.End},
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
func fromSeriesDTO(dto seriesDTO) *channel.Series {
	seasons := make([]channel.Season, len(dto.Seasons))
	for i, sDTO := range dto.Seasons {
		eps := make([]channel.Episode, len(sDTO.Episodes))
		for j, eDTO := range sDTO.Episodes {
			ep := channel.Episode{
				Source:   channel.Source{Kind: channel.SourceKind(eDTO.Source.Kind), ID: eDTO.Source.ID},
				Title:    eDTO.Title,
				Length:   eDTO.Length.Duration,
				Continue: eDTO.Continue,
			}
			if len(eDTO.Segments) > 0 {
				ep.Clips = make([]channel.Clip, len(eDTO.Segments))
				for k, sDTO := range eDTO.Segments {
					ep.Clips[k] = channel.Clip{
						Start: sDTO.Start.Duration,
						End:   sDTO.End.Duration,
					}
				}
			}
			eps[j] = ep
		}
		seasons[i] = channel.Season{Name: sDTO.Name, Episodes: eps}
	}
	return channel.NewSeries(dto.Name, seasons...)
}

// toStateDTO converts a channel.State to its JSON DTO.
func toStateDTO(s *channel.State) stateDTO {
	dto := stateDTO{ActiveSeries: s.ActiveSeries}
	s.EachSeriesState(func(id string, src channel.Source, pos time.Duration) {
		if dto.Series == nil {
			dto.Series = make(map[string]*seriesStateDTO)
		}
		dto.Series[id] = &seriesStateDTO{
			Source:   sourceDTO{Kind: string(src.Kind), ID: src.ID},
			Position: Duration{pos},
		}
	})
	return dto
}

// fromStateDTO converts a stateDTO back to a channel.State.
func fromStateDTO(dto stateDTO) *channel.State {
	state := channel.NewEmptyState()
	state.ActiveSeries = dto.ActiveSeries
	for id, ss := range dto.Series {
		src := channel.Source{Kind: channel.SourceKind(ss.Source.Kind), ID: ss.Source.ID}
		state.SetSeriesState(id, src, ss.Position.Duration)
	}
	return state
}
