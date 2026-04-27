package channel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testState(id string, pos time.Duration) *State {
	return &State{
		ID:       NewSource(id),
		Position: Duration{pos},
	}
}

func TestState_GetSetPosition(t *testing.T) {
	s := testState("a", 0)

	s.SetPosition(NewSource("a"), 30*time.Second)
	id, pos := s.Get()
	assert.Equal(t, "a", id.ID)
	assert.Equal(t, 30*time.Second, pos)
}

func TestState_SetPosition_IgnoresStale(t *testing.T) {
	s := testState("a", 10*time.Second)

	s.SetPosition(NewSource("b"), 99*time.Second)
	_, pos := s.Get()
	assert.Equal(t, 10*time.Second, pos, "stale SetPosition should be ignored")
}

func TestState_Jump(t *testing.T) {
	s := testState("a", 0)

	s.Jump(NewSource("b"), 5*time.Minute)
	id, pos := s.Get()
	assert.Equal(t, "b", id.ID)
	assert.Equal(t, 5*time.Minute, pos)
}

func TestState_Advance(t *testing.T) {
	s := testState("a", 30*time.Second)

	s.Advance(NewSource("a"), NewSource("b"), 0)
	id, pos := s.Get()
	assert.Equal(t, "b", id.ID)
	assert.Equal(t, time.Duration(0), pos)
}

func TestState_Advance_IgnoresStale(t *testing.T) {
	s := testState("a", 30*time.Second)

	s.Advance(NewSource("wrong"), NewSource("b"), 0)
	id, _ := s.Get()
	assert.Equal(t, "a", id.ID, "stale Advance should be ignored")
}

func TestState_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	sched := newTestSchedule(playlist("p1", "a", "b"))
	s := &State{
		path:     statePath,
		ID:       NewSource("b"),
		Position: Duration{2 * time.Minute},
	}
	require.NoError(t, s.Save())

	loaded := LoadState(statePath, sched)
	id, pos := loaded.Get()
	assert.Equal(t, "b", id.ID)
	assert.Equal(t, 2*time.Minute, pos)
}

func TestLoadState_MissingFile(t *testing.T) {
	sched := newTestSchedule(playlist("p1", "a"))
	s := LoadState("/nonexistent/state.json", sched)
	id, pos := s.Get()
	assert.Equal(t, "a", id.ID, "should fall back to first fragment")
	assert.Equal(t, time.Duration(0), pos)
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{bad`), 0644))

	sched := newTestSchedule(playlist("p1", "a"))
	s := LoadState(path, sched)
	id, _ := s.Get()
	assert.Equal(t, "a", id.ID, "should fall back to first fragment")
}

func TestLoadState_VideoRemovedFromSchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Save state pointing to "deleted"
	s := &State{
		path:     path,
		ID:       NewSource("deleted"),
		Position: Duration{time.Minute},
	}
	require.NoError(t, s.Save())

	// Schedule wraps, so the stale state is preserved as-is.
	sched := newTestSchedule(playlist("p1", "a"))
	loaded := LoadState(path, sched)
	id, pos := loaded.Get()
	assert.Equal(t, "deleted", id.ID)
	assert.Equal(t, time.Minute, pos)
}
