package channel_test

import (
	"slices"
	"testing"
	"time"

	"go-tv/internal/channel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEpisodeIterClips(t *testing.T) {
	src := channel.NewTestSource("s")
	tests := []struct {
		name      string
		episode   channel.Episode
		wantClips []channel.Clip
	}{
		{
			"no explicit clips yields one synthetic clip spanning full length",
			channel.NewEpisode(src, 30*time.Second),
			[]channel.Clip{channel.NewClip(0, 30*time.Second)},
		},
		{
			"single explicit clip",
			channel.NewEpisode(src, time.Minute, channel.NewClip(5*time.Second, 25*time.Second)),
			[]channel.Clip{channel.NewClip(5*time.Second, 25*time.Second)},
		},
		{
			"two explicit clips yielded in order",
			channel.NewEpisode(src, 90*time.Second,
				channel.NewClip(0, 30*time.Second),
				channel.NewClip(60*time.Second, 90*time.Second),
			),
			[]channel.Clip{channel.NewClip(0, 30*time.Second), channel.NewClip(60*time.Second, 90*time.Second)},
		},
		{
			"unsorted input is sorted ascending by start",
			channel.NewEpisode(src, 90*time.Second,
				channel.NewClip(60*time.Second, 90*time.Second),
				channel.NewClip(0, 30*time.Second),
			),
			[]channel.Clip{channel.NewClip(0, 30*time.Second), channel.NewClip(60*time.Second, 90*time.Second)},
		},
		{
			"three explicit clips",
			channel.NewEpisode(src, 3*time.Minute,
				channel.NewClip(0, 10*time.Second),
				channel.NewClip(60*time.Second, 70*time.Second),
				channel.NewClip(2*time.Minute, 3*time.Minute),
			),
			[]channel.Clip{
				channel.NewClip(0, 10*time.Second),
				channel.NewClip(60*time.Second, 70*time.Second),
				channel.NewClip(2*time.Minute, 3*time.Minute),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slices.Collect(tt.episode.IterClips())
			require.Equal(t, len(tt.wantClips), len(got))
			assert.True(t, slices.Equal(tt.wantClips, got))
		})
	}
}
