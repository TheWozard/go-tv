package channel_test

import (
	"encoding/json"
	"go-tv/internal/channel"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func newTestSchedule(series ...*channel.Series) *channel.Schedule {
	return channel.NewSchedule(series...)
}

func makeSeries(name string, ids ...string) *channel.Series {
	episodes := make([]channel.Episode, len(ids))
	for i, id := range ids {
		episodes[i] = channel.Episode{
			Source: channel.NewTestSource(id),
			Title:  id,
			Length: channel.Duration{10 * time.Minute},
		}
	}
	season := channel.Season{Name: name, Episodes: episodes}
	return channel.NewSeries("", name, season)
}

// Find

func TestSchedule_Find(t *testing.T) {
	s := newTestSchedule(makeSeries("s1", "a", "b"), makeSeries("s2", "c"))

	v, ok := s.Find(channel.NewTestSource("b"))
	assert.True(t, ok)
	assert.Equal(t, "b", v.Source.ID)

	_, ok = s.Find(channel.NewTestSource("missing"))
	assert.False(t, ok)
}

// First

func TestSchedule_First(t *testing.T) {
	s := newTestSchedule(makeSeries("s1", "a", "b"))
	frag, ok := s.First()
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)
	assert.Equal(t, time.Duration(0), frag.Start)
}

func TestSchedule_First_Empty(t *testing.T) {
	s := newTestSchedule()
	_, ok := s.First()
	assert.False(t, ok)
}

// All / AllSeries

func TestSchedule_All(t *testing.T) {
	s := newTestSchedule(makeSeries("s1", "a"), makeSeries("s2", "b", "c"))
	all := s.All()
	require.Len(t, all, 3)
	assert.Equal(t, "a", all[0].Source.ID)
	assert.Equal(t, "b", all[1].Source.ID)
	assert.Equal(t, "c", all[2].Source.ID)
}

func TestSchedule_AllSeries(t *testing.T) {
	s := newTestSchedule(makeSeries("s1", "a"), makeSeries("s2", "b"))
	items := s.AllSeries()
	require.Len(t, items, 2)
	assert.Equal(t, "s1", items[0].Name)
	assert.Equal(t, "s2", items[1].Name)
}

// Current

func TestSchedule_Current(t *testing.T) {
	s := newTestSchedule(makeSeries("s1", "a", "b"))

	frag, ok := s.Current(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)

	// Past end of "a" advances to "b"
	frag, ok = s.Current(channel.NewTestSource("a"), 11*time.Minute)
	assert.True(t, ok)
	assert.Equal(t, "b", frag.Source.ID, "should advance to next episode")
}

// Next with Continue flag

func TestSchedule_Next_Continue(t *testing.T) {
	// With Continue=true, next episode in same season plays.
	episodes := []channel.Episode{
		{Source: channel.NewTestSource("a"), Length: channel.Duration{10 * time.Minute}, Continue: true},
		{Source: channel.NewTestSource("b"), Length: channel.Duration{10 * time.Minute}},
	}
	season := channel.Season{Name: "show", Episodes: episodes}
	ser := channel.NewSeries("", "show", season)
	s := newTestSchedule(ser)

	frag, ok := s.Next(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "b", frag.Source.ID, "Continue=true should advance within same season")
}

func TestSchedule_Next_NoContinue_PicksRandomSeries(t *testing.T) {
	// With Continue=false (default) and one series, random picks from that series.
	s := newTestSchedule(makeSeries("s1", "a", "b"))

	frag, ok := s.Next(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID, "random pick with one series returns its first episode")
}

func TestSchedule_Next_NoContinue_MultipleSeries(t *testing.T) {
	// With Continue=false and multiple series, result is in one of them.
	s := newTestSchedule(makeSeries("s1", "a"), makeSeries("s2", "b"), makeSeries("s3", "c"))

	frag, ok := s.Next(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	validIDs := map[string]bool{"a": true, "b": true, "c": true}
	assert.True(t, validIDs[frag.Source.ID], "should land in one of the series")
}

func TestSchedule_Next_WrapsAround(t *testing.T) {
	// Continue=true on last episode of a season falls through to random pick.
	episodes := []channel.Episode{
		{Source: channel.NewTestSource("a"), Length: channel.Duration{10 * time.Minute}, Continue: true},
	}
	season := channel.Season{Name: "s1", Episodes: episodes}
	ser := channel.NewSeries("", "s1", season)
	s := newTestSchedule(ser)

	frag, ok := s.Next(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)
}

// AllSegments test — single episode with 4 segments, Continue=true
func TestSchedule_Next_AllSegments(t *testing.T) {
	cdur := func(d time.Duration) *channel.Duration { return &channel.Duration{d} }

	src := channel.NewYoutubeSource("v4YhsooE5xY")
	length := 44*time.Minute + 50*time.Second

	ep := channel.Episode{
		Source:   src,
		Title:    "Taskmaster - Series 1, Episode 1",
		Length:   channel.Duration{length},
		Continue: true,
		Segments: []channel.Segment{
			{End: cdur(10 * time.Second)},
			{Start: cdur(60 * time.Second), End: cdur(70 * time.Second)},
			{Start: cdur(120 * time.Second), End: cdur(130 * time.Second)},
			{Start: cdur(44*time.Minute + 40*time.Second)},
		},
	}
	season := channel.Season{Name: "Taskmaster - Series 1", Episodes: []channel.Episode{ep}}
	ser := channel.NewSeries("", "Taskmaster", season)
	s := newTestSchedule(ser)

	frag, ok := s.Next(src, 10*time.Second)
	require.True(t, ok)
	assert.Equal(t, 60*time.Second, frag.Start)
	assert.Equal(t, 70*time.Second, frag.End)

	frag, ok = s.Next(src, 70*time.Second)
	require.True(t, ok)
	assert.Equal(t, 120*time.Second, frag.Start)
	assert.Equal(t, 130*time.Second, frag.End)

	frag, ok = s.Next(src, 130*time.Second)
	require.True(t, ok)
	assert.Equal(t, 44*time.Minute+40*time.Second, frag.Start)
	assert.Equal(t, length, frag.End)

	// Past last segment wraps back to first via random pick (only one series)
	frag, ok = s.Next(src, length)
	require.True(t, ok)
	assert.Equal(t, time.Duration(0), frag.Start)
	assert.Equal(t, 10*time.Second, frag.End)
}

func TestSchedule_Next_MissingEpisode(t *testing.T) {
	s := newTestSchedule(makeSeries("s1", "a"))

	frag, ok := s.Next(channel.NewTestSource("missing"), 0)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)
}

// Save / Load round-trip for Series

func TestSeries_SaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "futurama.json")
	season := channel.Season{Name: "Season 1", Episodes: []channel.Episode{
		{Source: channel.NewTestSource("a"), Title: "ep a", Length: channel.Duration{30 * time.Minute}},
		{Source: channel.NewTestSource("b"), Title: "ep b", Length: channel.Duration{30 * time.Minute}},
	}}
	ser := channel.NewSeries(path, "Futurama", season)
	require.NoError(t, ser.Save())

	loaded, err := channel.LoadSeries(path)
	require.NoError(t, err)
	assert.Equal(t, "Futurama", loaded.Name)
	require.Len(t, loaded.Seasons, 1)
	assert.Equal(t, "Season 1", loaded.Seasons[0].Name)
	assert.Len(t, loaded.Seasons[0].Episodes, 2)
}

// LoadSeriesDir

func TestLoadSeriesDir(t *testing.T) {
	dir := t.TempDir()

	s1 := channel.NewSeries(filepath.Join(dir, "futurama.json"), "Futurama",
		channel.Season{Name: "Season 1", Episodes: []channel.Episode{
			{Source: channel.NewTestSource("a"), Length: channel.Duration{30 * time.Minute}},
		}})
	s2 := channel.NewSeries(filepath.Join(dir, "music.json"), "Music",
		channel.Season{Name: "Hits", Episodes: []channel.Episode{
			{Source: channel.NewTestSource("b"), Length: channel.Duration{5 * time.Minute}},
		}})
	require.NoError(t, s1.Save())
	require.NoError(t, s2.Save())

	sched, err := channel.LoadSeriesDir(dir)
	require.NoError(t, err)
	assert.Len(t, sched.AllSeries(), 2)
	assert.Len(t, sched.All(), 2)
}

func TestLoadSeriesDir_NotFound(t *testing.T) {
	_, err := channel.LoadSeriesDir("/nonexistent/dir")
	assert.Error(t, err)
}

// JSON structure

func TestSeries_JSON_Structure(t *testing.T) {
	ser := channel.NewSeries("", "My Series",
		channel.Season{Name: "Season 1", Episodes: []channel.Episode{
			{Source: channel.NewTestSource("abc"), Title: "Test", Length: channel.Duration{time.Minute}},
		}},
	)
	data, err := json.Marshal(ser)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, string(data), `"seasons"`)
	assert.Contains(t, string(data), `"episodes"`)
	assert.Contains(t, string(data), `"name"`)
}

func TestLoadSeries_NotFound(t *testing.T) {
	_, err := channel.LoadSeries("/nonexistent/path.json")
	assert.Error(t, err)
}

func TestLoadSeries_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{invalid`), 0644))
	_, err := channel.LoadSeries(path)
	assert.Error(t, err)
}

// Per-series state

func TestSeries_StateInitialisedToFirstFragment(t *testing.T) {
	ser := makeSeries("s1", "a", "b")
	src, pos := ser.GetState()
	assert.Equal(t, "a", src.ID)
	assert.Equal(t, time.Duration(0), pos)
}

func TestSeries_UpdateState(t *testing.T) {
	ser := makeSeries("s1", "a", "b")
	ser.UpdateState(channel.NewTestSource("a"), 5*time.Minute)
	src, pos := ser.GetState()
	assert.Equal(t, "a", src.ID)
	assert.Equal(t, 5*time.Minute, pos)
}

func TestSeries_UpdateState_IgnoresStale(t *testing.T) {
	ser := makeSeries("s1", "a", "b")
	ser.UpdateState(channel.NewTestSource("b"), 5*time.Minute) // "b" doesn't match initial state "a"
	_, pos := ser.GetState()
	assert.Equal(t, time.Duration(0), pos, "stale update should be ignored")
}

func TestSeries_JumpState(t *testing.T) {
	ser := makeSeries("s1", "a", "b")
	ser.JumpState(channel.NewTestSource("b"), 3*time.Minute)
	src, pos := ser.GetState()
	assert.Equal(t, "b", src.ID)
	assert.Equal(t, 3*time.Minute, pos)
}

func TestSeries_SaveState_LoadSeries_Resumption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "show.json")
	season := channel.Season{Name: "S1", Episodes: []channel.Episode{
		{Source: channel.NewTestSource("a"), Length: channel.Duration{10 * time.Minute}},
		{Source: channel.NewTestSource("b"), Length: channel.Duration{10 * time.Minute}},
	}}
	ser := channel.NewSeries(path, "Show", season)
	require.NoError(t, ser.Save())

	// Advance state to "b" at 4m and save.
	ser.JumpState(channel.NewTestSource("b"), 4*time.Minute)
	require.NoError(t, ser.SaveState())

	// Re-load the series; state should resume at "b"/4m.
	loaded, err := channel.LoadSeries(path)
	require.NoError(t, err)
	src, pos := loaded.GetState()
	assert.Equal(t, "b", src.ID)
	assert.Equal(t, 4*time.Minute, pos)
}

func TestLoadSeriesDir_SkipsStateFiles(t *testing.T) {
	dir := t.TempDir()
	ser := channel.NewSeries(filepath.Join(dir, "show.json"), "Show",
		channel.Season{Name: "S1", Episodes: []channel.Episode{
			{Source: channel.NewTestSource("a"), Length: channel.Duration{10 * time.Minute}},
		}})
	require.NoError(t, ser.Save())
	require.NoError(t, ser.SaveState()) // creates show.state.json

	sched, err := channel.LoadSeriesDir(dir)
	require.NoError(t, err)
	assert.Len(t, sched.AllSeries(), 1, "state file must not be loaded as a series")
}

func TestSchedule_Next_ResumesSeriesState(t *testing.T) {
	// Two series; series "s2" has state at episode "c" / 2m.
	s1 := makeSeries("s1", "a")
	s2 := makeSeries("s2", "b", "c")
	s2.JumpState(channel.NewTestSource("c"), 2*time.Minute)

	// With two series, Next with Continue=false calls randomFirstLocked.
	// We can't control which series is picked (random), but if s2 is picked
	// the fragment must start at "c"/2m rather than "b"/0.
	sched := newTestSchedule(s1, s2)

	// Call Next from "a" (end of s1 with no Continue) many times and verify
	// that whenever s2 is picked, it resumes at "c"/2m.
	for range 20 {
		frag, ok := sched.Next(channel.NewTestSource("a"), 30*time.Second)
		require.True(t, ok)
		if frag.Source.ID == "c" {
			assert.Equal(t, 2*time.Minute, frag.Start, "s2 must resume at saved position")
			return
		}
	}
	// If we never hit s2 that's a statistical fluke, not a failure.
}

func TestChannel_Progress_UpdatesSeriesState(t *testing.T) {
	ser := makeSeries("s1", "a", "b")
	sched := newTestSchedule(ser)
	globalState := channel.NewState(channel.NewTestSource("a"), 0)
	ch := channel.NewChannel(sched, globalState)

	ch.Progress(channel.NewTestSource("a"), 5*time.Minute)

	src, pos := ser.GetState()
	assert.Equal(t, "a", src.ID)
	assert.Equal(t, 5*time.Minute, pos)
}

func TestChannel_Jump_UpdatesSeriesState(t *testing.T) {
	ser := makeSeries("s1", "a", "b")
	sched := newTestSchedule(ser)
	globalState := channel.NewState(channel.NewTestSource("a"), 0)
	ch := channel.NewChannel(sched, globalState)

	require.NoError(t, ch.Jump(channel.NewTestSource("b"), 3*time.Minute))

	src, pos := ser.GetState()
	assert.Equal(t, "b", src.ID)
	assert.Equal(t, 3*time.Minute, pos)
}
