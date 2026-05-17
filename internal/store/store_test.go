package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-tv/internal/channel"
	"go-tv/internal/store"
)

// helpers

func makeSeries(name string, ids ...string) *channel.Series {
	episodes := make([]channel.Episode, len(ids))
	for i, id := range ids {
		episodes[i] = channel.Episode{
			Source: channel.NewTestSource(id),
			Title:  id,
			Length: 10 * time.Minute,
		}
	}
	season := channel.Season{Name: name, Episodes: episodes}
	return channel.NewSeries(name, season)
}

func newTestSchedule(series ...*channel.Series) *channel.Schedule {
	return channel.NewSchedule(series...)
}

// Series Save / Load round-trip

func TestSeries_SaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "futurama.json")
	season := channel.Season{Name: "Season 1", Episodes: []channel.Episode{
		{Source: channel.NewTestSource("a"), Title: "ep a", Length: 30 * time.Minute},
		{Source: channel.NewTestSource("b"), Title: "ep b", Length: 30 * time.Minute},
	}}
	ser := channel.NewSeries("Futurama", season)
	require.NoError(t, store.SaveSeries(path, ser))

	loaded, err := store.LoadSeries(path)
	require.NoError(t, err)
	assert.Equal(t, "Futurama", loaded.Name)
	require.Len(t, loaded.Seasons, 1)
	assert.Equal(t, "Season 1", loaded.Seasons[0].Name)
	assert.Len(t, loaded.Seasons[0].Episodes, 2)
}

func TestLoadSeries_NotFound(t *testing.T) {
	_, err := store.LoadSeries("/nonexistent/path.json")
	assert.Error(t, err)
}

func TestLoadSeries_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{invalid`), 0644))
	_, err := store.LoadSeries(path)
	assert.Error(t, err)
}

// State Save / Load round-trip

func TestState_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	schedule := newTestSchedule(makeSeries("p1", "a", "b"))
	s := channel.NewStateFor("p1", channel.NewTestSource("b"), 2*time.Minute)
	require.NoError(t, store.SaveState(statePath, s))

	loaded := store.LoadState(statePath, schedule)
	source, pos := loaded.Get()
	assert.Equal(t, "b", source.ID)
	assert.Equal(t, 2*time.Minute, pos)
}

func TestLoadState_MissingFile(t *testing.T) {
	schedule := newTestSchedule(makeSeries("p1", "a"))
	s := store.LoadState("/nonexistent/state.json", schedule)
	source, pos := s.Get()
	assert.Equal(t, "a", source.ID, "should fall back to first fragment")
	assert.Equal(t, time.Duration(0), pos)
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{bad`), 0644))

	sched := newTestSchedule(makeSeries("p1", "a"))
	s := store.LoadState(path, sched)
	source, _ := s.Get()
	assert.Equal(t, "a", source.ID, "should fall back to first fragment")
}

func TestLoadState_VideoRemovedFromSchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := channel.NewStateFor("p1", channel.NewTestSource("deleted"), time.Minute)
	require.NoError(t, store.SaveState(path, s))

	sched := newTestSchedule(makeSeries("p1", "a"))
	loaded := store.LoadState(path, sched)
	source, pos := loaded.Get()
	assert.Equal(t, "a", source.ID)
	assert.Equal(t, time.Duration(0), pos)
}

// LoadSeriesDir

func TestLoadSeriesDir(t *testing.T) {
	dir := t.TempDir()

	s1 := channel.NewSeries("Futurama", channel.Season{Name: "Season 1", Episodes: []channel.Episode{
		{Source: channel.NewTestSource("a"), Length: 30 * time.Minute},
	}})
	s2 := channel.NewSeries("Music", channel.Season{Name: "Hits", Episodes: []channel.Episode{
		{Source: channel.NewTestSource("b"), Length: 5 * time.Minute},
	}})
	require.NoError(t, store.SaveSeries(filepath.Join(dir, "futurama.json"), s1))
	require.NoError(t, store.SaveSeries(filepath.Join(dir, "music.json"), s2))

	serFiles, err := store.LoadSeriesDir(dir)
	require.NoError(t, err)
	assert.Len(t, serFiles, 2)

	sched := channel.NewSchedule(serFiles[0].Series, serFiles[1].Series)
	assert.Len(t, sched.AllSeries(), 2)
	assert.Len(t, sched.All(), 2)
}

func TestLoadSeriesDir_NotFound(t *testing.T) {
	_, err := store.LoadSeriesDir("/nonexistent/dir")
	assert.Error(t, err)
}

func TestLoadSeriesDir_SkipsStateFiles(t *testing.T) {
	dir := t.TempDir()
	ser := channel.NewSeries("Show", channel.Season{Name: "S1", Episodes: []channel.Episode{
		{Source: channel.NewTestSource("a"), Length: 10 * time.Minute},
	}})
	require.NoError(t, store.SaveSeries(filepath.Join(dir, "show.json"), ser))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "show.state.json"), []byte(`{}`), 0644))

	serFiles, err := store.LoadSeriesDir(dir)
	require.NoError(t, err)
	assert.Len(t, serFiles, 1, "state file must not be loaded as a series")
}
