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

func newTestSchedule(playlists ...channel.Playlist) *channel.Schedule {
	return &channel.Schedule{Playlists: playlists}
}

func playlist(name string, ids ...string) channel.Playlist {
	videos := make([]channel.Video, len(ids))
	for i, id := range ids {
		videos[i] = channel.Video{
			Source: channel.NewTestSource(id),
			Title:  id,
			Length: channel.Duration{10 * time.Minute},
		}
	}
	return channel.Playlist{Name: name, Videos: videos}
}

// Find

func TestSchedule_Find(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"), playlist("p2", "c"))

	v, ok := s.Find(channel.NewTestSource("b"))
	assert.True(t, ok)
	assert.Equal(t, "b", v.Source.ID)

	_, ok = s.Find(channel.NewTestSource("missing"))
	assert.False(t, ok)
}

// First

func TestSchedule_First(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))
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

// All / AllItems

func TestSchedule_All(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a"), playlist("p2", "b", "c"))
	all := s.All()
	require.Len(t, all, 3)
	assert.Equal(t, "a", all[0].Source.ID)
	assert.Equal(t, "b", all[1].Source.ID)
	assert.Equal(t, "c", all[2].Source.ID)
}

func TestSchedule_AllItems(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a"), playlist("p2", "b"))
	items := s.AllItems()
	require.Len(t, items, 2)
	assert.Equal(t, "p1", items[0].Name)
	assert.Equal(t, "p2", items[1].Name)
}

// Update

func TestSchedule_Update(t *testing.T) {
	s := newTestSchedule(playlist("old", "a"))
	s.Update([]channel.Playlist{playlist("new", "x", "y")})
	assert.Len(t, s.All(), 2)
	assert.Equal(t, "new", s.AllItems()[0].Name)
}

// Current / Next navigation

func TestSchedule_Current(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))

	frag, ok := s.Current(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)

	// Past end of "a" advances to "b"
	frag, ok = s.Current(channel.NewTestSource("a"), 11*time.Minute)
	assert.True(t, ok)
	assert.Equal(t, "b", frag.Source.ID, "should advance to next video")
}

func TestSchedule_Next(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))

	// Mid-video advances to next video
	frag, ok := s.Next(channel.NewTestSource("a"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "b", frag.Source.ID, "should advance to next video")
	assert.Equal(t, time.Duration(0), frag.Start)
}

func TestSchedule_Next_WrapsAround(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))

	// Past last video wraps to first
	frag, ok := s.Next(channel.NewTestSource("b"), 30*time.Second)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID, "should wrap to first video")
}

// TestSchedule_Next_AllSegments mirrors schedule.json (single video, 4 segments)
// and verifies that calling Next from each segment's start position visits every
// segment in order, wrapping back to segment 1 after the last.
func TestSchedule_Next_AllSegments(t *testing.T) {
	cdur := func(d time.Duration) *channel.Duration { return &channel.Duration{d} }

	src := channel.NewYoutubeSource("v4YhsooE5xY")
	length := 44*time.Minute + 50*time.Second

	v := channel.Video{
		Source: src,
		Title:  "Taskmaster - Series 1, Episode 1",
		Length: channel.Duration{length},
		Segments: []channel.Segment{
			{End: cdur(10 * time.Second)},                                                      // seg1: 0 → 10s
			{Start: cdur(60 * time.Second), End: cdur(70 * time.Second)},                       // seg2: 1m → 1m10s
			{Start: cdur(120 * time.Second), End: cdur(130 * time.Second)},                     // seg3: 2m → 2m10s
			{Start: cdur(44*time.Minute + 40*time.Second)},                                     // seg4: 44m40s → end
		},
	}
	s := newTestSchedule(channel.Playlist{Name: "Taskmaster - Series 1", Videos: []channel.Video{v}})

	// From end of seg1 (10s) → seg2
	frag, ok := s.Next(src, 10*time.Second)
	require.True(t, ok)
	assert.Equal(t, 60*time.Second, frag.Start)
	assert.Equal(t, 70*time.Second, frag.End)

	// From end of seg2 (70s) → seg3
	frag, ok = s.Next(src, 70*time.Second)
	require.True(t, ok)
	assert.Equal(t, 120*time.Second, frag.Start)
	assert.Equal(t, 130*time.Second, frag.End)

	// From end of seg3 (130s) → seg4
	frag, ok = s.Next(src, 130*time.Second)
	require.True(t, ok)
	assert.Equal(t, 44*time.Minute+40*time.Second, frag.Start)
	assert.Equal(t, length, frag.End)

	// From end of seg4 (length) → wraps to seg1
	frag, ok = s.Next(src, length)
	require.True(t, ok)
	assert.Equal(t, time.Duration(0), frag.Start)
	assert.Equal(t, 10*time.Second, frag.End)
}

func TestSchedule_Next_MissingVideo(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a"))

	// Missing video wraps to first
	frag, ok := s.Next(channel.NewTestSource("missing"), 0)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)
}

// Save / Load round-trip

func TestSchedule_SaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedule.json")
	s := channel.NewSchedule(path, playlist("p1", "a", "b"))
	require.NoError(t, s.Save())

	loaded, err := channel.LoadSchedule(path)
	require.NoError(t, err)
	assert.Len(t, loaded.All(), 2)
	assert.Equal(t, "a", loaded.All()[0].Source.ID)
}

func TestLoadSchedule_NotFound(t *testing.T) {
	_, err := channel.LoadSchedule("/nonexistent/path.json")
	assert.Error(t, err)
}

func TestLoadSchedule_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{invalid`), 0644))
	_, err := channel.LoadSchedule(path)
	assert.Error(t, err)
}

// channel.Schedule JSON structure

func TestSchedule_JSON_Structure(t *testing.T) {
	s := channel.NewSchedule(
		"",
		channel.Playlist{Name: "My channel.Playlist", Videos: []channel.Video{
			{Source: channel.NewTestSource("abc"), Title: "Test", Length: channel.Duration{time.Minute}},
		}},
	)
	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, string(data), `"playlists"`)
}
