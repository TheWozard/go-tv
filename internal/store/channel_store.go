package store

import (
	"time"

	"go-tv/internal/channel"
	"go-tv/internal/channel/mutation"
)

// ChannelStore wraps a Channel with persistence logic.
// Non-saving methods are promoted from the embedded *channel.Channel.
type ChannelStore struct {
	*channel.Channel
	seriesPaths map[string]string
	statePath   string
}

func NewChannelStore(ch *channel.Channel, seriesPaths map[string]string, statePath string) *ChannelStore {
	return &ChannelStore{
		Channel:     ch,
		seriesPaths: seriesPaths,
		statePath:   statePath,
	}
}

func (cs *ChannelStore) Next(source channel.Source, position time.Duration) error {
	if err := cs.Channel.Next(source, position); err != nil {
		return err
	}
	return cs.saveState()
}

func (cs *ChannelStore) Jump(source channel.Source, position time.Duration) error {
	if err := cs.Channel.Jump(source, position); err != nil {
		return err
	}
	return cs.saveState()
}

func (cs *ChannelStore) RenameSeason(seriesID, oldName, newName string) error {
	if err := mutation.RenameSeason(cs.Channel.Schedule(), seriesID, oldName, newName); err != nil {
		return err
	}
	return cs.saveSeries(seriesID)
}

func (cs *ChannelStore) ReorderSeries(seriesID string, orders []mutation.SeasonOrder) error {
	if err := mutation.ReorderSeries(cs.Channel.Schedule(), seriesID, orders); err != nil {
		return err
	}
	return cs.saveSeries(seriesID)
}

func (cs *ChannelStore) ApplyCuts(source channel.Source, cuts []mutation.CutRange) (*channel.Episode, error) {
	ep, err := mutation.ApplyCuts(cs.Channel.Schedule(), source, cuts)
	if err != nil {
		return nil, err
	}
	sr := cs.Channel.Schedule().SeriesOf(ep.Source)
	if sr == nil {
		return ep, nil
	}
	if err := cs.saveSeries(sr.ID); err != nil {
		return nil, err
	}
	return ep, nil
}

func (cs *ChannelStore) ToggleSeriesActive(seriesID string) error {
	cs.Channel.ToggleSeriesActive(seriesID)
	return cs.saveState()
}

func (cs *ChannelStore) SetEpisodeMode(source channel.Source, mode channel.EpisodeMode) (*channel.Episode, error) {
	ep, err := mutation.SetEpisodeMode(cs.Channel.Schedule(), source, mode)
	if err != nil {
		return nil, err
	}
	sr := cs.Channel.Schedule().SeriesOf(ep.Source)
	if sr == nil {
		return ep, nil
	}
	if err := cs.saveSeries(sr.ID); err != nil {
		return nil, err
	}
	return ep, nil
}

func (cs *ChannelStore) SetSeriesMode(seriesID string, mode channel.SeriesMode) error {
	if err := cs.Channel.SetSeriesMode(seriesID, mode); err != nil {
		return err
	}
	return cs.saveSeries(seriesID)
}

func (cs *ChannelStore) ToggleShuffle() error {
	cs.Channel.SetShuffle(!cs.Channel.State().Shuffle)
	return cs.saveState()
}

func (cs *ChannelStore) saveSeries(id string) error {
	path, ok := cs.seriesPaths[id]
	if !ok {
		return nil
	}
	for _, sr := range cs.Channel.AllSeries() {
		if sr.ID == id {
			return SaveSeries(path, sr)
		}
	}
	return nil
}

func (cs *ChannelStore) saveState() error {
	return SaveState(cs.statePath, cs.Channel.State())
}
