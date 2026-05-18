package channel_test

import (
	"testing"
	"time"

	"go-tv/internal/channel"

	"github.com/stretchr/testify/assert"
)

func TestScheduleCurrentSegmentAt(t *testing.T) {
	tests := []struct {
		name     string
		schedule *channel.Schedule
		source   channel.Source
		position time.Duration
		wantSeg  channel.Segment
		wantOK   bool
	}{
		{
			"source not in schedule",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))),
			srcB, 0,
			channel.Segment{}, false,
		},
		{
			"single season: position within episode",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))),
			srcA, 30 * time.Second,
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"single season Single mode: past end returns false",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))),
			srcA, time.Minute,
			channel.Segment{}, false,
		},
		{
			"two seasons: position past first season advances to second",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcA, time.Minute,
			channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"two seasons: position within first season stays in first",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcA, 30 * time.Second,
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"Single mode: past last season returns false",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcB, time.Minute,
			channel.Segment{}, false,
		},
		{
			"LoopMode: past last season wraps to first",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.LoopMode,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcB, time.Minute,
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"three seasons: advances to correct next season",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcC, time.Minute)),
			)),
			srcB, time.Minute,
			channel.Segment{Source: srcC, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"episode with clips: position between clips skips to next",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(
				channel.NewEpisode(srcA, 90*time.Second,
					channel.NewClip(0, 30*time.Second),
					channel.NewClip(60*time.Second, 90*time.Second),
				),
			))),
			srcA, 45 * time.Second,
			channel.Segment{Source: srcA, Clip: channel.NewClip(60*time.Second, 90*time.Second)},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seg, ok := tt.schedule.CurrentSegmentAt(tt.source, tt.position, false, func(string) bool { return true })
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantSeg, seg)
			}
		})
	}
}

func TestScheduleNextSegmentAt(t *testing.T) {
	tests := []struct {
		name     string
		schedule *channel.Schedule
		source   channel.Source
		position time.Duration
		wantSeg  channel.Segment
		wantOK   bool
	}{
		{
			"source not in schedule",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))),
			srcB, 0,
			channel.Segment{}, false,
		},
		{
			"single season Single mode: no clip boundary returns false",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)))),
			srcA, 30 * time.Second,
			channel.Segment{}, false,
		},
		{
			"single season with clips: returns next boundary within season",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single, channel.NewAnonymousSeason(
				channel.NewEpisode(srcA, 90*time.Second,
					channel.NewClip(0, 30*time.Second),
					channel.NewClip(60*time.Second, 90*time.Second),
				),
			))),
			srcA, 5 * time.Second,
			channel.Segment{Source: srcA, Clip: channel.NewClip(60*time.Second, 90*time.Second)},
			true,
		},
		{
			"two seasons: past last clip advances to next season",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, 90*time.Second,
					channel.NewClip(0, 30*time.Second),
					channel.NewClip(60*time.Second, 90*time.Second),
				)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcA, 65 * time.Second,
			channel.Segment{Source: srcB, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"Single mode: no more clips in last season returns false",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.Single,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcB, 30 * time.Second,
			channel.Segment{}, false,
		},
		{
			"LoopMode: no more clips in last season wraps to first",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.LoopMode,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
			)),
			srcB, 30 * time.Second,
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
		{
			"LoopMode: wraps to first season skipping exhausted seasons",
			channel.NewSchedule(channel.NewAnonymousSeries(channel.LoopMode,
				channel.NewAnonymousSeason(channel.NewEpisode(srcA, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcB, time.Minute)),
				channel.NewAnonymousSeason(channel.NewEpisode(srcC, time.Minute)),
			)),
			srcC, 30 * time.Second,
			channel.Segment{Source: srcA, Clip: channel.NewClip(0, time.Minute)},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seg, ok := tt.schedule.NextSegmentAt(tt.source, tt.position, false, func(string) bool { return true })
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantSeg, seg)
			}
		})
	}
}
