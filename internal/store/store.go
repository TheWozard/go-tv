package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"go-tv/internal/channel"
)

// SeriesFile pairs a loaded Series with the file it came from.
type SeriesFile struct {
	Series *channel.Series
	Path   string
}

// LoadSeries reads and decodes a Series from the given JSON file.
func LoadSeries(path string) (*channel.Series, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var dto seriesDTO
	if err := json.NewDecoder(f).Decode(&dto); err != nil {
		return nil, err
	}
	return fromSeriesDTO(dto), nil
}

// SaveSeries writes s to path as pretty-printed JSON.
func SaveSeries(path string, s *channel.Series) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(toSeriesDTO(s))
}

// LoadSeriesDir loads all .json series files from dir.
// State files (*.state.json) are skipped for backwards compatibility.
func LoadSeriesDir(dir string) ([]SeriesFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []SeriesFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".state.json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		s, err := LoadSeries(path)
		if err != nil {
			return nil, err
		}
		out = append(out, SeriesFile{Series: s, Path: path})
	}
	return out, nil
}

// LoadState reads persisted state from path. If the file is missing, corrupt,
// or refers to a position no longer valid in the schedule, it falls back to
// the first playable fragment.
func LoadState(path string, schedule *channel.Schedule) *channel.State {
	makeDefault := func() *channel.State {
		if first, ok := schedule.First(); ok {
			if ser := schedule.SeriesOf(first.Source); ser != nil {
				return channel.NewStateFor(ser.ID(), first.Source, first.Start)
			}
		}
		return channel.NewEmptyState()
	}

	f, err := os.Open(path)
	if err != nil {
		return makeDefault()
	}
	defer func() { _ = f.Close() }()

	var dto stateDTO
	if err := json.NewDecoder(f).Decode(&dto); err != nil {
		return makeDefault()
	}

	if dto.ActiveSeries == "" || dto.Series[dto.ActiveSeries] == nil {
		return makeDefault()
	}

	state := fromStateDTO(dto)
	src, pos := state.Get()
	if current, ok := schedule.Current(src, pos); ok {
		if !current.Source.Equal(src) || current.Start > pos {
			state.Jump(state.ActiveSeries, current.Source, current.Start)
		}
		return state
	}
	return makeDefault()
}

// SaveState persists s to path as pretty-printed JSON.
func SaveState(path string, s *channel.State) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(toStateDTO(s))
}
