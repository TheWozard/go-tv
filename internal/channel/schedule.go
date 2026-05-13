package channel

import (
	"encoding/json"
	"iter"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Season is a named group of episodes within a [Series].
type Season struct {
	Name     string    `json:"name"`
	Episodes []Episode `json:"episodes"`
}

// Series is a single TV series/show backed by its own JSON file.
// It is persisted to disk and all methods are safe for concurrent use.
type Series struct {
	mu      sync.RWMutex
	path    string
	state   *State // per-series playback position
	Name    string   `json:"name"`
	Seasons []Season `json:"seasons"`
}

// NewSeries creates a Series with the given path, name, and seasons.
func NewSeries(path string, name string, seasons ...Season) *Series {
	s := &Series{path: path, Name: name, Seasons: seasons}
	s.initState()
	return s
}

// LoadSeries reads and decodes a Series from the given JSON file.
func LoadSeries(path string) (*Series, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var s Series
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	s.path = path
	s.initState()
	return &s, nil
}

// statePath returns the path to the per-series state file, or "" if no path is set.
func (s *Series) statePath() string {
	if s.path == "" {
		return ""
	}
	return strings.TrimSuffix(s.path, ".json") + ".state.json"
}

// firstFragment returns the first playable fragment in this series.
func (s *Series) firstFragment() (Fragment, bool) {
	for _, season := range s.Seasons {
		for _, ep := range season.Episodes {
			if f, ok := ep.Next(0); ok {
				return f, true
			}
		}
	}
	return Fragment{}, false
}

// initState loads per-series state from disk or initialises it to the first fragment.
func (s *Series) initState() {
	sp := s.statePath()
	if sp != "" {
		if st := s.loadStateFile(sp); st != nil {
			s.state = st
			return
		}
	}
	f, ok := s.firstFragment()
	src, pos := Source{}, time.Duration(0)
	if ok {
		src, pos = f.Source, f.Start
	}
	s.state = &State{path: sp, Source: src, Position: Duration{pos}}
}

// loadStateFile attempts to read and validate a state file for this series.
// Returns nil if the file is missing, unparseable, or references an unknown source.
func (s *Series) loadStateFile(sp string) *State {
	f, err := os.Open(sp)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var st State
	if err := json.NewDecoder(f).Decode(&st); err != nil {
		return nil
	}
	for _, season := range s.Seasons {
		for _, ep := range season.Episodes {
			if ep.Source.Equal(st.Source) {
				st.path = sp
				return &st
			}
		}
	}
	return nil
}

// resumeFragment returns the fragment to resume playback from for this series.
// Falls back to the first fragment if the saved state is invalid.
// Must be called with no series lock held (acquires none itself).
func (s *Series) resumeFragment() (Fragment, bool) {
	if s.state == nil {
		return s.firstFragment()
	}
	src, pos := s.state.Get()
	if src.Equal((Source{})) {
		return s.firstFragment()
	}
	for _, season := range s.Seasons {
		for _, ep := range season.Episodes {
			if ep.Source.Equal(src) {
				if f, ok := ep.Current(pos); ok {
					f.Start = pos // resume at the exact saved position, not segment start
					return f, true
				}
				// Position past last segment; fall back to series start.
				break
			}
		}
	}
	return s.firstFragment()
}

// GetState returns the saved source and playback position for this series.
func (s *Series) GetState() (Source, time.Duration) {
	if s.state == nil {
		return Source{}, 0
	}
	return s.state.Get()
}

// UpdateState records the current playback position for this series.
// Ignored if source doesn't match the current series state source (stale).
func (s *Series) UpdateState(source Source, pos time.Duration) {
	if s.state != nil {
		s.state.SetPosition(source, pos)
	}
}

// JumpState unconditionally moves the series state to source at pos.
func (s *Series) JumpState(source Source, pos time.Duration) {
	if s.state != nil {
		s.state.Jump(source, pos)
	}
}

// SaveState persists the per-series state to disk.
func (s *Series) SaveState() error {
	if s.state == nil {
		return nil
	}
	return s.state.Save()
}

// Save writes the series to disk as pretty-printed JSON.
func (s *Series) Save() (err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := os.Create(s.path)
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
	return enc.Encode(s)
}

// AllSeasons returns a shallow copy of the seasons slice, safe for concurrent use.
func (s *Series) AllSeasons() []Season {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Season, len(s.Seasons))
	copy(out, s.Seasons)
	return out
}

// UpdateSeasons replaces the seasons list atomically.
func (s *Series) UpdateSeasons(seasons []Season) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Seasons = seasons
}

// SetFilePath sets the file path for persistence.
func (s *Series) SetFilePath(path string) {
	s.path = path
}

// episodeLocation records the position of an episode within the schedule hierarchy.
type episodeLocation struct {
	serIdx, seasonIdx, epIdx int
}

// Schedule coordinates playback across multiple [Series].
// All methods are safe for concurrent use.
type Schedule struct {
	mu     sync.RWMutex
	series []*Series
	idx    map[Source]episodeLocation // O(1) lookup by source
}

// NewSchedule creates a Schedule from the given Series.
func NewSchedule(series ...*Series) *Schedule {
	s := &Schedule{series: series}
	s.buildIndexLocked()
	return s
}

// LoadSeriesDir loads all .json files from dir as Series and returns a Schedule.
func LoadSeriesDir(dir string) (*Schedule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var series []*Series
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".state.json") {
			continue
		}
		s, err := LoadSeries(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		series = append(series, s)
	}
	return NewSchedule(series...), nil
}

// buildIndexLocked rebuilds the source→location index.
// Must be called without the schedule lock held (or during construction).
func (s *Schedule) buildIndexLocked() {
	idx := make(map[Source]episodeLocation)
	for si, ser := range s.series {
		for ki, season := range ser.Seasons {
			for ei, ep := range season.Episodes {
				idx[ep.Source] = episodeLocation{si, ki, ei}
			}
		}
	}
	s.idx = idx
}

// rebuildIndex acquires the write lock and rebuilds the index.
// Call after any mutation to episode membership or order.
func (s *Schedule) rebuildIndex() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buildIndexLocked()
}

// AllSeries returns all series, safe for concurrent use.
func (s *Schedule) AllSeries() []*Series {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Series, len(s.series))
	copy(out, s.series)
	return out
}

// FindSeries returns the series with the given name, or nil if not found.
func (s *Schedule) FindSeries(name string) *Series {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ser := range s.series {
		if ser.Name == name {
			return ser
		}
	}
	return nil
}

// iterEpisodes returns an iterator over all episodes across all series in playback order.
// Must be called with schedule lock held.
func (s *Schedule) iterEpisodes() iter.Seq[Episode] {
	return func(yield func(Episode) bool) {
		for _, ser := range s.series {
			for _, season := range ser.Seasons {
				for _, ep := range season.Episodes {
					if !yield(ep) {
						return
					}
				}
			}
		}
	}
}

// Find returns the episode matching the given source.
func (s *Schedule) Find(id Source) (*Episode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if loc, ok := s.idx[id]; ok {
		ep := s.series[loc.serIdx].Seasons[loc.seasonIdx].Episodes[loc.epIdx]
		return &ep, true
	}
	return nil, false
}

// First returns the first possible [Fragment] for the current schedule.
func (s *Schedule) First() (Fragment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firstLocked()
}

// firstLocked returns the first playable fragment. Must be called with schedule lock held.
func (s *Schedule) firstLocked() (Fragment, bool) {
	for ep := range s.iterEpisodes() {
		if f, ok := ep.Next(0); ok {
			return f, true
		}
	}
	return Fragment{}, false
}

// Current returns the fragment containing the given position within the
// identified episode. If the position falls in a gap between segments, the next
// segment's fragment is returned.
func (s *Schedule) Current(id Source, position time.Duration) (Fragment, bool) {
	return s.findFragment(id, position, Episode.Current)
}

// Next returns the fragment that follows the given position. If the current
// episode has no further segments, behavior depends on the episode's Continue
// flag: if set, the next episode in the same season is played; otherwise a
// series is chosen at random and its first episode plays.
// Wraps to the beginning when no suitable next is found.
func (s *Schedule) Next(id Source, position time.Duration) (Fragment, bool) {
	if id.Equal(Source{}) {
		return s.First()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	loc, found := s.idx[id]
	if !found {
		return s.firstLocked()
	}

	ep := s.series[loc.serIdx].Seasons[loc.seasonIdx].Episodes[loc.epIdx]

	// More segments within the same episode?
	if f, ok := ep.Next(position); ok {
		return f, true
	}

	// Episode finished. If Continue is set, play the next episode in this season.
	if ep.Continue {
		season := s.series[loc.serIdx].Seasons[loc.seasonIdx]
		for i := loc.epIdx + 1; i < len(season.Episodes); i++ {
			if f, ok := season.Episodes[i].Next(0); ok {
				return f, true
			}
		}
		// End of season; fall through to random series pick.
	}

	return s.randomFirstLocked()
}

// randomFirstLocked picks a random series and resumes from its saved state.
// Must be called with schedule lock held.
func (s *Schedule) randomFirstLocked() (Fragment, bool) {
	if len(s.series) == 0 {
		return Fragment{}, false
	}
	start := rand.Intn(len(s.series))
	for i := range s.series {
		ser := s.series[(start+i)%len(s.series)]
		if f, ok := ser.resumeFragment(); ok {
			return f, true
		}
	}
	return Fragment{}, false
}

// SeriesOf returns the series that contains the given source, or nil.
func (s *Schedule) SeriesOf(source Source) *Series {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if loc, ok := s.idx[source]; ok {
		return s.series[loc.serIdx]
	}
	return nil
}

func (s *Schedule) findFragment(source Source, position time.Duration, check func(Episode, time.Duration) (Fragment, bool)) (Fragment, bool) {
	if source.Equal(Source{}) {
		return s.First()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	loc, ok := s.idx[source]
	if !ok {
		return s.firstLocked()
	}

	ep := s.series[loc.serIdx].Seasons[loc.seasonIdx].Episodes[loc.epIdx]
	if f, ok := check(ep, position); ok {
		return f, true
	}

	// Position past all segments: advance through subsequent episodes in order.
	for si := loc.serIdx; si < len(s.series); si++ {
		kiStart, eiStart := 0, 0
		if si == loc.serIdx {
			kiStart = loc.seasonIdx
		}
		for ki := kiStart; ki < len(s.series[si].Seasons); ki++ {
			if si == loc.serIdx && ki == loc.seasonIdx {
				eiStart = loc.epIdx + 1
			} else {
				eiStart = 0
			}
			for ei := eiStart; ei < len(s.series[si].Seasons[ki].Episodes); ei++ {
				if f, ok := check(s.series[si].Seasons[ki].Episodes[ei], 0); ok {
					return f, true
				}
			}
		}
	}
	return s.firstLocked()
}

// All returns the flat playback order across all series, safe for concurrent use.
func (s *Schedule) All() []Episode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Episode
	for ep := range s.iterEpisodes() {
		out = append(out, ep)
	}
	return out
}
