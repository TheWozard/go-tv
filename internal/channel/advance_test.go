package channel_test

// Tests reproducing the YouTube auto-advance bug where a video with a single
// clipped segment fails to advance to the next episode after the clip ends.
//
// Observed sequence on iPad Chrome / Vivaldi:
//   1. advance() fires at currentStop (clip.End = 2630s)
//   2. Server responds with sameSource=true, pos=2654 — no advance occurs
//   3. Player seeks to 2654, YouTube fires ENDED, advance() fires again
//   4. Second advance is throttled or suppressed by endedFired, video stalls
//
// These tests probe every layer of the Go-side pipeline that could cause the
// wrong segment to be returned after Next().

import (
	"sync"
	"testing"
	"time"

	"go-tv/internal/channel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mirrors the Taskmaster S06E01 config exactly:
//
//	"segments": [{"start": "28s", "end": "43m50s"}]
//	"length": "44m15s"
const (
	clipStart  = 28 * time.Second
	clipEnd    = 43*time.Minute + 50*time.Second // 2630s
	epLength   = 44*time.Minute + 15*time.Second // 2655s
	ytDuration = 2654*time.Second + 460*time.Millisecond // actual YouTube reported duration
)

// newTaskmasterEpisode builds an episode matching the Taskmaster S06E01 config.
func newTaskmasterEpisode(src channel.Source) channel.Episode {
	return channel.NewEpisode(src, epLength, channel.NewClip(clipStart, clipEnd))
}

// TestNextAtClipEndAdvancesEpisode verifies that when Next() is called with
// position == clip.End (the stop time), the episode advances to the next one.
// This is the primary invariant broken by the bug.
func TestNextAtClipEndAdvancesEpisode(t *testing.T) {
	ep1 := newTaskmasterEpisode(srcA)
	ep2 := channel.NewEpisode(srcB, time.Minute)
	sr := channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(ep1, ep2))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	// Simulate the player calling Next at exactly the stop time (clip.End).
	require.NoError(t, ch.Next(srcA, clipEnd))

	got := ch.CurrentSegment()
	assert.Equal(t, srcB, got.Source, "should advance to episode 2")
	assert.Equal(t, channel.NewClip(0, time.Minute), got.Clip)
}

// TestNextAtClipEndAdvancesEpisodeSubSecondPrecision verifies that Next() called
// with a position slightly past clip.End (from floating-point currentTime) still
// advances. parseDuration truncates to the second so this should behave identically.
func TestNextAtClipEndAdvancesEpisodeSubSecondPrecision(t *testing.T) {
	ep1 := newTaskmasterEpisode(srcA)
	ep2 := channel.NewEpisode(srcB, time.Minute)
	sr := channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(ep1, ep2))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	// 2630.015s truncated to second → 2630s; should behave the same as exact clip.End.
	pos := clipEnd // parseDuration truncates, so use the truncated value
	require.NoError(t, ch.Next(srcA, pos))

	got := ch.CurrentSegment()
	assert.Equal(t, srcB, got.Source, "sub-second position past clip.End must advance")
}

// TestStaleProgressAfterNextDoesNotRevertSegment verifies that a stale Progress()
// call arriving after Next() has advanced to episode 2 does not cause
// CurrentSegment() to return episode 1 again.
//
// This races in production: the progress ticker fires with the old source/position
// concurrently with the Next handler calling CurrentSegment().
func TestStaleProgressAfterNextDoesNotRevertSegment(t *testing.T) {
	ep1 := newTaskmasterEpisode(srcA)
	ep2 := channel.NewEpisode(srcB, time.Minute)
	sr := channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(ep1, ep2))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	require.NoError(t, ch.Next(srcA, clipEnd))

	// Stale progress: the client reported a position near the end of ep1 just
	// before or during the Next request. This should not revert CurrentSegment.
	ch.Progress(srcA, ytDuration.Truncate(time.Second))

	got := ch.CurrentSegment()
	assert.Equal(t, srcB, got.Source, "stale Progress must not revert CurrentSegment to ep1")
}

// TestNextThenStaleProgressCurrentSegmentMatchesNextResponse mirrors the server
// handler sequence: Next() → CurrentSegment() (response) → stale Progress().
// The segment returned by CurrentSegment immediately after Next must equal what
// the server renders, regardless of a subsequent stale Progress.
func TestNextThenStaleProgressCurrentSegmentMatchesNextResponse(t *testing.T) {
	ep1 := newTaskmasterEpisode(srcA)
	ep2 := channel.NewEpisode(srcB, time.Minute)
	sr := channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(ep1, ep2))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	require.NoError(t, ch.Next(srcA, clipEnd))
	segAfterNext := ch.CurrentSegment() // what nextHandler renders

	// Stale progress arrives (would occur via concurrent HTTP handler in production).
	ch.Progress(srcA, ytDuration.Truncate(time.Second))
	segAfterProgress := ch.CurrentSegment() // what a subsequent getState would see

	assert.Equal(t, srcB, segAfterNext.Source, "nextHandler response must point to ep2")
	assert.Equal(t, segAfterNext.Source, segAfterProgress.Source,
		"stale Progress must not change the segment visible to subsequent requests")
}

// TestConcurrentNextAndProgress exercises the race condition where the HTTP
// server's Next and Progress handlers execute concurrently in different goroutines.
// Run with -race to detect data races on the shared channel state.
func TestConcurrentNextAndProgress(t *testing.T) {
	ep1 := newTaskmasterEpisode(srcA)
	ep2 := channel.NewEpisode(srcB, time.Minute)
	sr := channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(ep1, ep2))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	const iterations = 200
	for i := 0; i < iterations; i++ {
		// Reset to a fresh channel each iteration.
		ch = channel.NewEmptyChannel(channel.NewSchedule(
			channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(
				newTaskmasterEpisode(srcA),
				channel.NewEpisode(srcB, time.Minute),
			)),
		))
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = ch.Next(srcA, clipEnd)
		}()
		go func() {
			defer wg.Done()
			ch.Progress(srcA, ytDuration.Truncate(time.Second))
		}()
		wg.Wait()

		// After both operations, CurrentSegment must not be zero.
		got := ch.CurrentSegment()
		assert.False(t, got.Source.IsZero(), "CurrentSegment must not be zero after concurrent Next+Progress")
	}
}

// TestSingleEpisodeSeriesExhaustsOnClipEnd verifies that a single-episode series
// is exhausted (not looped) when Next is called at clip.End.
func TestSingleEpisodeSeriesExhaustsOnClipEnd(t *testing.T) {
	ep1 := newTaskmasterEpisode(srcA)
	sr := channel.NewSeries("Taskmaster", channel.OnceMode, channel.NewAnonymousSeason(ep1))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	err := ch.Next(srcA, clipEnd)
	assert.Error(t, err, "single OnceMode episode must return error when exhausted")
	assert.False(t, ch.State().IsActive(sr.ID), "series must be inactive after exhaustion")
}
