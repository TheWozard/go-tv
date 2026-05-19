package mutation_test

import (
	"testing"
	"time"

	"go-tv/internal/channel"
	"go-tv/internal/channel/mutation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func src(id string) channel.Source { return channel.NewTestSource(id) }

func ep(id string, length time.Duration, clips ...channel.Clip) channel.Episode {
	return channel.NewEpisode(src(id), length, clips...)
}

func buildSchedule(series ...*channel.Series) *channel.Schedule {
	return channel.NewSchedule(series...)
}

func TestRenameSeasonSuccess(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("Season 1", ep("a", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.RenameSeason(sc, sr.ID, "Season 1", "Year One")
	require.NoError(t, err)
	assert.Equal(t, "Year One", sc.Series[0].Seasons[0].Name)
}

func TestRenameSeasonUnknownSeriesReturnsError(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.RenameSeason(sc, "nonexistent", "S1", "X")
	assert.Error(t, err)
}

func TestRenameSeasonUnknownSeasonReturnsError(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.RenameSeason(sc, sr.ID, "S2", "X")
	assert.Error(t, err)
}

func TestReorderSeriesReordersEpisodesWithinSeason(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute), ep("b", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.ReorderSeries(sc, sr.ID, []mutation.SeasonOrder{
		{Name: "S1", EpisodeIDs: []string{"b", "a"}},
	})
	require.NoError(t, err)

	eps := sc.Series[0].Seasons[0].Episodes
	require.Len(t, eps, 2)
	assert.Equal(t, src("b"), eps[0].Source)
	assert.Equal(t, src("a"), eps[1].Source)
}

func TestReorderSeriesMergeSeasonsIntoOne(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute)),
		channel.NewSeason("S2", ep("b", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.ReorderSeries(sc, sr.ID, []mutation.SeasonOrder{
		{Name: "Combined", EpisodeIDs: []string{"a", "b"}},
	})
	require.NoError(t, err)

	seasons := sc.Series[0].Seasons
	require.Len(t, seasons, 1)
	assert.Equal(t, "Combined", seasons[0].Name)
	assert.Len(t, seasons[0].Episodes, 2)
}

func TestReorderSeriesSkipsMissingIDs(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute), ep("b", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.ReorderSeries(sc, sr.ID, []mutation.SeasonOrder{
		{Name: "S1", EpisodeIDs: []string{"a", "ghost"}},
	})
	require.NoError(t, err)

	eps := sc.Series[0].Seasons[0].Episodes
	require.Len(t, eps, 1)
	assert.Equal(t, src("a"), eps[0].Source)
}

func TestReorderSeriesUnknownSeriesReturnsError(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute)),
	)
	sc := buildSchedule(sr)

	err := mutation.ReorderSeries(sc, "no-such-id", []mutation.SeasonOrder{
		{Name: "S1", EpisodeIDs: []string{"a"}},
	})
	assert.Error(t, err)
}

func TestReorderSeriesRebuildsIndex(t *testing.T) {
	// After reorder the schedule index must be updated so Next/Jump still work.
	sr := channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute), ep("b", time.Minute)),
	)
	sc := buildSchedule(sr)
	ch := channel.NewChannel(sc, channel.NewEmptyState())

	err := mutation.ReorderSeries(sc, sr.ID, []mutation.SeasonOrder{
		{Name: "S1", EpisodeIDs: []string{"b", "a"}},
	})
	require.NoError(t, err)

	// src("b") is now episode 0 → Next from it should return src("a").
	err = ch.Next(src("b"), time.Minute)
	require.NoError(t, err)
	seg := ch.CurrentSegment()
	assert.Equal(t, src("a"), seg.Source)
}

func TestApplyCutsNoRemainingContent(t *testing.T) {
	// Cutting the entire video → no clips → IterClips yields synthetic clip.
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 60*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 0, End: 60 * time.Second},
	})
	require.NoError(t, err)
	assert.Empty(t, updated.Clips)
}

func TestApplyCutsMiddleCut(t *testing.T) {
	// Cut [30-60] from 90s video → clips [0-30] and [60-90].
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 90*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 30 * time.Second, End: 60 * time.Second},
	})
	require.NoError(t, err)
	require.Len(t, updated.Clips, 2)
	assert.Equal(t, channel.NewClip(0, 30*time.Second), updated.Clips[0])
	assert.Equal(t, channel.NewClip(60*time.Second, 90*time.Second), updated.Clips[1])
}

func TestApplyCutsTailCut(t *testing.T) {
	// Cut [60-90] from 90s video → clip [0-60].
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 90*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 60 * time.Second, End: 90 * time.Second},
	})
	require.NoError(t, err)
	require.Len(t, updated.Clips, 1)
	assert.Equal(t, channel.NewClip(0, 60*time.Second), updated.Clips[0])
}

func TestApplyCutsHeadCut(t *testing.T) {
	// Cut [0-30] from 90s video → clip [30-90].
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 90*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 0, End: 30 * time.Second},
	})
	require.NoError(t, err)
	require.Len(t, updated.Clips, 1)
	assert.Equal(t, channel.NewClip(30*time.Second, 90*time.Second), updated.Clips[0])
}

func TestApplyCutsMultipleNonOverlappingCuts(t *testing.T) {
	// Cuts [0-10] and [20-30] from 40s → clips [10-20] and [30-40].
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 40*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 0, End: 10 * time.Second},
		{Start: 20 * time.Second, End: 30 * time.Second},
	})
	require.NoError(t, err)
	require.Len(t, updated.Clips, 2)
	assert.Equal(t, channel.NewClip(10*time.Second, 20*time.Second), updated.Clips[0])
	assert.Equal(t, channel.NewClip(30*time.Second, 40*time.Second), updated.Clips[1])
}

func TestApplyCutsOverlappingCuts(t *testing.T) {
	// Cuts [10-50] and [30-70] overlap; union=[10-70]; 90s video → clips [0-10] and [70-90].
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 90*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 10 * time.Second, End: 50 * time.Second},
		{Start: 30 * time.Second, End: 70 * time.Second},
	})
	require.NoError(t, err)
	require.Len(t, updated.Clips, 2)
	assert.Equal(t, channel.NewClip(0, 10*time.Second), updated.Clips[0])
	assert.Equal(t, channel.NewClip(70*time.Second, 90*time.Second), updated.Clips[1])
}

func TestApplyCutsUnsortedCuts(t *testing.T) {
	// Cuts [{60-90}, {0-30}] unsorted on a 90s video → clip [30-60].
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", 90*time.Second)),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), []mutation.CutRange{
		{Start: 60 * time.Second, End: 90 * time.Second},
		{Start: 0, End: 30 * time.Second},
	})
	require.NoError(t, err)
	require.Len(t, updated.Clips, 1, "unsorted cuts should produce the same result as sorted cuts")
	assert.Equal(t, channel.NewClip(30*time.Second, 60*time.Second), updated.Clips[0])
}

func TestApplyCutsUnknownVideoReturnsError(t *testing.T) {
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1", ep("a", time.Minute)),
	))

	_, err := mutation.ApplyCuts(sc, src("no-such-video"), []mutation.CutRange{
		{Start: 0, End: 10 * time.Second},
	})
	assert.Error(t, err)
}

func TestApplyCutsEmptyCutsRemovesClips(t *testing.T) {
	// Episode already has clips; applying zero cuts → clips reset to nil
	// (IterClips then yields the synthetic full-length clip).
	sc := buildSchedule(channel.NewSeries("Show", channel.OnceMode,
		channel.NewSeason("S1",
			ep("a", 60*time.Second, channel.NewClip(0, 30*time.Second)),
		),
	))

	updated, err := mutation.ApplyCuts(sc, src("a"), nil)
	require.NoError(t, err)
	assert.Empty(t, updated.Clips)
}
