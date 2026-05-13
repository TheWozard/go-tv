package channel_test

import (
	"go-tv/internal/channel"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testState(id string, pos time.Duration) *channel.State {
	return channel.NewState(channel.NewTestSource(id), pos)
}

func TestState_GetSetPosition(t *testing.T) {
	s := testState("a", 0)

	s.SetPosition(channel.NewTestSource("a"), 30*time.Second)
	source, pos := s.Get()
	assert.Equal(t, "a", source.ID)
	assert.Equal(t, 30*time.Second, pos)
}

func TestState_SetPosition_IgnoresStale(t *testing.T) {
	s := testState("a", 10*time.Second)

	s.SetPosition(channel.NewTestSource("b"), 99*time.Second)
	_, pos := s.Get()
	assert.Equal(t, 10*time.Second, pos, "stale SetPosition should be ignored")
}

func TestState_Jump(t *testing.T) {
	s := testState("a", 0)

	s.Jump(channel.NewTestSource("b"), 5*time.Minute)
	source, pos := s.Get()
	assert.Equal(t, "b", source.ID)
	assert.Equal(t, 5*time.Minute, pos)
}

func TestState_Advance(t *testing.T) {
	s := testState("a", 30*time.Second)

	s.Advance(channel.NewTestSource("a"), channel.NewTestSource("b"), 0)
	source, pos := s.Get()
	assert.Equal(t, "b", source.ID)
	assert.Equal(t, time.Duration(0), pos)
}

func TestState_Advance_IgnoresStale(t *testing.T) {
	s := testState("a", 30*time.Second)

	s.Advance(channel.NewTestSource("wrong"), channel.NewTestSource("b"), 0)
	source, _ := s.Get()
	assert.Equal(t, "a", source.ID, "stale Advance should be ignored")
}

func TestState_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	schedule := newTestSchedule(makeSeries("p1", "a", "b"))
	s := testState("b", 2*time.Minute)
	s.SetFilePath(statePath)
	require.NoError(t, s.Save())

	loaded := channel.LoadState(statePath, schedule)
	source, pos := loaded.Get()
	assert.Equal(t, "b", source.ID)
	assert.Equal(t, 2*time.Minute, pos)
}

func TestLoadState_MissingFile(t *testing.T) {
	schedule := newTestSchedule(makeSeries("p1", "a"))
	s := channel.LoadState("/nonexistent/state.json", schedule)
	source, pos := s.Get()
	assert.Equal(t, "a", source.ID, "should fall back to first fragment")
	assert.Equal(t, time.Duration(0), pos)
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{bad`), 0644))

	sched := newTestSchedule(makeSeries("p1", "a"))
	s := channel.LoadState(path, sched)
	source, _ := s.Get()
	assert.Equal(t, "a", source.ID, "should fall back to first fragment")
}

func TestLoadState_VideoRemovedFromSchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save state pointing to "deleted"
	s := testState("deleted", time.Minute)
	s.SetFilePath(path)
	require.NoError(t, s.Save())

	// Video not in schedule, falls back to first fragment.
	sched := newTestSchedule(makeSeries("p1", "a"))
	loaded := channel.LoadState(path, sched)
	source, pos := loaded.Get()
	assert.Equal(t, "a", source.ID)
	assert.Equal(t, time.Duration(0), pos)
}
