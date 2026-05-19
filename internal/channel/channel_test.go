package channel_test

import (
	"testing"
	"time"

	"go-tv/internal/channel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelCurrentSegment(t *testing.T) {
	tests := []struct {
		name string
		ch   func() *channel.Channel
		want channel.Segment
	}{
		{
			"empty state falls back to first segment",
			func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
				))))
			},
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			"state mid-episode returns that episode's segment",
			func() *channel.Channel {
				sc := channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute))))
				return channel.NewChannel(sc, channel.NewStateFor("", srcA, 30*time.Second))
			},
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			"position between clips returns next clip",
			func() *channel.Channel {
				ep := channel.NewEpisode(srcA, 90*time.Second,
					channel.NewClip(0, 30*time.Second),
					channel.NewClip(60*time.Second, 90*time.Second),
				)
				sc := channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(ep)))
				return channel.NewChannel(sc, channel.NewStateFor("", srcA, 45*time.Second))
			},
			channel.Segment{Source: srcA, Clip: channel.NewClip(60*time.Second, 90*time.Second)},
		},
		{
			"unknown source falls back to first segment",
			func() *channel.Channel {
				sc := channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute))))
				return channel.NewChannel(sc, channel.NewStateFor("", channel.NewTestSource("unknown"), 0))
			},
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ch().CurrentSegment())
		})
	}
}

func TestChannelNext(t *testing.T) {
	tests := []struct {
		name    string
		ch      func() *channel.Channel
		src     channel.Source
		pos     time.Duration
		wantErr bool
		wantSeg channel.Segment
		check   func(*testing.T, *channel.Channel)
	}{
		{
			name: "advances within episode clips",
			ch: func() *channel.Channel {
				ep := channel.NewEpisode(srcA, 90*time.Second,
					channel.NewClip(0, 30*time.Second),
					channel.NewClip(60*time.Second, 90*time.Second),
				)
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(ep))))
			},
			src:     srcA,
			pos:     30 * time.Second,
			wantSeg: channel.Segment{Source: srcA, Clip: channel.NewClip(60*time.Second, 90*time.Second)},
		},
		{
			name: "advances to next episode",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
					channel.NewEpisode(srcB, time.Minute),
				))))
			},
			src:     srcA,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			name: "advances to next season",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode,
					channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
					channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
				)))
			},
			src:     srcA,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			name: "end of series advances to next series",
			ch: func() *channel.Channel {
				srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
				srB := channel.NewSeries("ShowB", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)))
				return channel.NewEmptyChannel(channel.NewSchedule(srA, srB))
			},
			src:     srcA,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
			check: func(t *testing.T, ch *channel.Channel) {
				srA := ch.AllSeries()[0]
				assert.False(t, ch.State().IsActive(srA.ID), "exhausted series should be inactive")
			},
		},
		{
			name: "all series exhausted returns error",
			ch: func() *channel.Channel {
				sr := channel.NewSeries("Show", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
				return channel.NewEmptyChannel(channel.NewSchedule(sr))
			},
			src:     srcA,
			pos:     time.Minute,
			wantErr: true,
			check: func(t *testing.T, ch *channel.Channel) {
				sr := ch.AllSeries()[0]
				assert.False(t, ch.State().IsActive(sr.ID), "exhausted series should be inactive")
			},
		},
		{
			name: "LoopMode wraps to first episode",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.LoopMode,
					channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
					channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
				)))
			},
			src:     srcB,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			name: "LoopMode single season wraps and series stays active",
			ch: func() *channel.Channel {
				sr := channel.NewSeries("Show", channel.LoopMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
					channel.NewEpisode(srcB, time.Minute),
				))
				return channel.NewEmptyChannel(channel.NewSchedule(sr))
			},
			src:     srcB,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
			check: func(t *testing.T, ch *channel.Channel) {
				sr := ch.AllSeries()[0]
				assert.True(t, ch.State().IsActive(sr.ID), "looping series must not be exhausted")
			},
		},
		{
			name: "unknown source returns error",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
				))))
			},
			src:     channel.NewTestSource("unknown"),
			pos:     0,
			wantErr: true,
		},
		{
			name: "EpisodeContinueMode in shuffle series stays in same series",
			ch: func() *channel.Channel {
				epA := channel.NewEpisode(srcA, time.Minute).WithMode(channel.EpisodeContinueMode)
				epB := channel.NewEpisode(srcB, time.Minute)
				srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(epA, epB))
				srOther := channel.NewSeries("Other", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcC, time.Minute)))
				ch := channel.NewEmptyChannel(channel.NewSchedule(srA, srOther))
				ch.SetShuffle(true)
				return ch
			},
			src:     srcA,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			name: "shuffle same series picked continues from next episode",
			ch: func() *channel.Channel {
				epA := channel.NewEpisode(srcA, time.Minute)
				epB := channel.NewEpisode(srcB, time.Minute)
				// Only one series so shuffle always picks it; must advance to epB, not restart at epA.
				srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(epA, epB))
				ch := channel.NewEmptyChannel(channel.NewSchedule(srA))
				ch.SetShuffle(true)
				return ch
			},
			src:     srcA,
			pos:     time.Minute,
			wantSeg: channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			name: "shuffle exhausts finished series and picks from remaining",
			ch: func() *channel.Channel {
				epA := channel.NewEpisode(srcA, time.Minute)
				epB := channel.NewEpisode(srcB, time.Minute)
				srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(epA))
				srB := channel.NewSeries("ShowB", channel.OnceMode, channel.NewAnonymousSeason(epB))
				ch := channel.NewEmptyChannel(channel.NewSchedule(srA, srB))
				ch.SetShuffle(true)
				return ch
			},
			src: srcA,
			pos: time.Minute,
			check: func(t *testing.T, ch *channel.Channel) {
				assert.False(t, ch.CurrentSegment().Source.IsZero())
				idA := ch.AllSeries()[0].ID
				idB := ch.AllSeries()[1].ID
				assert.False(t, ch.State().IsActive(idA), "ShowA should be exhausted")
				assert.True(t, ch.State().IsActive(idB), "ShowB should remain active")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.ch()
			err := ch.Next(tt.src, tt.pos)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.wantSeg != (channel.Segment{}) {
					assert.Equal(t, tt.wantSeg, ch.CurrentSegment())
				}
			}
			if tt.check != nil {
				tt.check(t, ch)
			}
		})
	}
}

func TestChannelJump(t *testing.T) {
	tests := []struct {
		name    string
		ch      func() *channel.Channel
		src     channel.Source
		pos     time.Duration
		wantErr bool
		wantSeg channel.Segment
	}{
		{
			name: "sets state to target episode",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
					channel.NewEpisode(srcB, time.Minute),
				))))
			},
			src:     srcB,
			pos:     30 * time.Second,
			wantSeg: channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
		},
		{
			name: "position between clips snaps to next clip",
			ch: func() *channel.Channel {
				ep := channel.NewEpisode(srcA, 90*time.Second,
					channel.NewClip(0, 30*time.Second),
					channel.NewClip(60*time.Second, 90*time.Second),
				)
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(ep))))
			},
			src:     srcA,
			pos:     45 * time.Second,
			wantSeg: channel.Segment{Source: srcA, Clip: channel.NewClip(60*time.Second, 90*time.Second)},
		},
		{
			name: "unknown source returns error",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
				))))
			},
			src:     channel.NewTestSource("unknown"),
			wantErr: true,
		},
		{
			// Single mode, 1 episode; position at end → CurrentSegmentAt returns false.
			name: "position past end returns error",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
				))))
			},
			src:     srcA,
			pos:     time.Minute,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.ch()
			err := ch.Jump(tt.src, tt.pos)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantSeg, ch.CurrentSegment())
			}
		})
	}
}

func TestChannelProgress(t *testing.T) {
	tests := []struct {
		name    string
		ch      func() *channel.Channel
		src     channel.Source
		pos     time.Duration
		wantSrc channel.Source
		wantPos time.Duration
	}{
		{
			name: "records position for known source",
			ch: func() *channel.Channel {
				sr := channel.NewSeries("Show", channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
					channel.NewEpisode(srcB, time.Minute),
				))
				return channel.NewEmptyChannel(channel.NewSchedule(sr))
			},
			src:     srcA,
			pos:     20 * time.Second,
			wantSrc: srcA,
			wantPos: 20 * time.Second,
		},
		{
			name: "unknown source is a no-op",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
				))))
			},
			src:     channel.NewTestSource("unknown"),
			pos:     10 * time.Second,
			wantSrc: channel.Source{},
			wantPos: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.ch()
			ch.Progress(tt.src, tt.pos)
			gotSrc, gotPos := ch.SeriesStateOf(tt.src)
			assert.Equal(t, tt.wantSrc, gotSrc)
			assert.Equal(t, tt.wantPos, gotPos)
		})
	}
}

func TestChannelToggleSeriesActive(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	assert.True(t, ch.State().IsActive(sr.ID))
	ch.ToggleSeriesActive(sr.ID)
	assert.False(t, ch.State().IsActive(sr.ID))
	ch.ToggleSeriesActive(sr.ID)
	assert.True(t, ch.State().IsActive(sr.ID))
}

func TestChannelToggleSeriesActivePicksNewWhenCurrentlyActive(t *testing.T) {
	srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
	srB := channel.NewSeries("ShowB", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)))
	ch := channel.NewEmptyChannel(channel.NewSchedule(srA, srB))

	require.NoError(t, ch.Jump(srcA, 0))
	require.Equal(t, srA.ID, ch.State().ActiveSeries)

	ch.ToggleSeriesActive(srA.ID)

	assert.False(t, ch.State().IsActive(srA.ID))
	assert.Equal(t, srB.ID, ch.State().ActiveSeries, "should switch to a remaining active series")
}

// TestShuffleInitializesNewSeriesState verifies that when shuffleActive picks a series
// that has never been played, it initialises the series to its first episode.
// Without this, CurrentSegment falls back to schedule.First() which may return the
// wrong episode (e.g. the current episode of a still-active earlier series).
func TestShuffleInitializesNewSeriesState(t *testing.T) {
	srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
	srB := channel.NewSeries("ShowB", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)))
	// srA is first in schedule order; srB has no prior playback state.
	ch := channel.NewEmptyChannel(channel.NewSchedule(srA, srB))

	require.NoError(t, ch.Jump(srcA, 0))

	// Deactivating the current series forces shuffleActive to pick srB (only remaining active).
	ch.ToggleSeriesActive(srA.ID)

	// srB must be active and its state must be initialised to its first episode.
	assert.Equal(t, srB.ID, ch.State().ActiveSeries)
	gotSrc, gotPos := ch.SeriesStateOf(srcB)
	assert.Equal(t, srcB, gotSrc, "new series must be initialised to its first episode source")
	assert.Equal(t, time.Duration(0), gotPos)
	assert.Equal(t, channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)}, ch.CurrentSegment())
}

func TestChannelSeriesStateOf(t *testing.T) {
	tests := []struct {
		name    string
		ch      func() *channel.Channel
		src     channel.Source
		wantSrc channel.Source
		wantPos time.Duration
	}{
		{
			name: "unknown source returns zero state",
			ch: func() *channel.Channel {
				return channel.NewEmptyChannel(channel.NewSchedule(channel.NewAnonymousSeries(channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
				))))
			},
			src:     channel.NewTestSource("unknown"),
			wantSrc: channel.Source{},
			wantPos: 0,
		},
		{
			name: "tracks progress after Progress call",
			ch: func() *channel.Channel {
				sr := channel.NewSeries("Show", channel.OnceMode, channel.NewAnonymousSeason(
					channel.NewEpisode(srcA, time.Minute),
					channel.NewEpisode(srcB, time.Minute),
				))
				ch := channel.NewEmptyChannel(channel.NewSchedule(sr))
				ch.Progress(srcA, 42*time.Second)
				return ch
			},
			src:     srcA,
			wantSrc: srcA,
			wantPos: 42 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotPos := tt.ch().SeriesStateOf(tt.src)
			assert.Equal(t, tt.wantSrc, gotSrc)
			assert.Equal(t, tt.wantPos, gotPos)
		})
	}
}

func TestChannelProgressDoesNotOverrideActiveSeries(t *testing.T) {
	srA := channel.NewSeries("ShowA", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
	srB := channel.NewSeries("ShowB", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)))
	ch := channel.NewEmptyChannel(channel.NewSchedule(srA, srB))

	// Establish srB as the active series via an explicit Jump.
	require.NoError(t, ch.Jump(srcB, 0))
	require.Equal(t, srB.ID, ch.State().ActiveSeries)

	// A stale Progress from srcA (e.g. in-flight at the time of the Jump) must
	// not revert ActiveSeries back to srA.
	ch.Progress(srcA, 30*time.Second)
	assert.Equal(t, srB.ID, ch.State().ActiveSeries, "stale Progress must not override Jump")

	// The position for srA is still recorded for resume purposes.
	gotSrc, gotPos := ch.SeriesStateOf(srcA)
	assert.Equal(t, srcA, gotSrc)
	assert.Equal(t, 30*time.Second, gotPos)
}

func TestChannelOnceModeExhaustesSeries(t *testing.T) {
	sr := channel.NewSeries("Show", channel.OnceMode, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))
	ch := channel.NewEmptyChannel(channel.NewSchedule(sr))

	err := ch.Next(srcA, time.Minute)
	assert.Error(t, err, "OnceMode should exhaust series and return error when no more content")
	assert.False(t, ch.State().IsActive(sr.ID), "exhausted series must be inactive")
}
