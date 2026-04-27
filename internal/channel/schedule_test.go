package channel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSchedule(playlists ...Playlist) *Schedule {
	return &Schedule{Playlists: playlists}
}

func playlist(name string, ids ...string) Playlist {
	videos := make([]Video, len(ids))
	for i, id := range ids {
		videos[i] = Video{
			Source: NewSource(id),
			Title:  id,
			Length: Duration{10 * time.Minute},
		}
	}
	return Playlist{Name: name, Videos: videos}
}

// Find

func TestSchedule_Find(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"), playlist("p2", "c"))

	v, ok := s.Find(NewSource("b"))
	assert.True(t, ok)
	assert.Equal(t, "b", v.Source.ID)

	_, ok = s.Find(NewSource("missing"))
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
	s.Update([]Playlist{playlist("new", "x", "y")})
	assert.Len(t, s.All(), 2)
	assert.Equal(t, "new", s.AllItems()[0].Name)
}

// Current / Next navigation

func TestSchedule_Current(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))

	frag, ok := s.Current(NewSource("a"), sec(30))
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)

	// Past end of "a" advances to "b"
	frag, ok = s.Current(NewSource("a"), 11*time.Minute)
	assert.True(t, ok)
	assert.Equal(t, "b", frag.Source.ID, "should advance to next video")
}

func TestSchedule_Next(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))

	// Mid-video advances to next video
	frag, ok := s.Next(NewSource("a"), sec(30))
	assert.True(t, ok)
	assert.Equal(t, "b", frag.Source.ID, "should advance to next video")
	assert.Equal(t, time.Duration(0), frag.Start)
}

func TestSchedule_Next_WrapsAround(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a", "b"))

	// Past last video wraps to first
	frag, ok := s.Next(NewSource("b"), sec(30))
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID, "should wrap to first video")
}

func TestSchedule_Next_MissingVideo(t *testing.T) {
	s := newTestSchedule(playlist("p1", "a"))

	// Missing video wraps to first
	frag, ok := s.Next(NewSource("missing"), 0)
	assert.True(t, ok)
	assert.Equal(t, "a", frag.Source.ID)
}

// Save / Load round-trip

func TestSchedule_SaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedule.json")
	s := &Schedule{
		path:      path,
		Playlists: []Playlist{playlist("p1", "a", "b")},
	}
	require.NoError(t, s.Save())

	loaded, err := LoadSchedule(path)
	require.NoError(t, err)
	assert.Len(t, loaded.All(), 2)
	assert.Equal(t, "a", loaded.All()[0].Source.ID)
}

func TestLoadSchedule_NotFound(t *testing.T) {
	_, err := LoadSchedule("/nonexistent/path.json")
	assert.Error(t, err)
}

func TestLoadSchedule_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{invalid`), 0644))
	_, err := LoadSchedule(path)
	assert.Error(t, err)
}

// Schedule JSON structure

func TestSchedule_JSON_Structure(t *testing.T) {
	s := &Schedule{
		Playlists: []Playlist{
			{Name: "My Playlist", Videos: []Video{
				{Source: NewSource("abc"), Title: "Test", Length: Duration{time.Minute}},
			}},
		},
	}
	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, string(data), `"playlists"`)
}
