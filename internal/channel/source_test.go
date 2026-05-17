package channel_test

import (
	"testing"

	"go-tv/internal/channel"

	"github.com/stretchr/testify/assert"
)

func TestSourceEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b channel.Source
		want bool
	}{
		{"same kind and id", channel.NewTestSource("x"), channel.NewTestSource("x"), true},
		{"different id", channel.NewTestSource("x"), channel.NewTestSource("y"), false},
		{"different kind same id", channel.NewTestSource("x"), channel.NewYoutubeSource("x"), false},
		{"both zero", channel.Source{}, channel.Source{}, true},
		{"zero vs non-zero", channel.Source{}, channel.NewTestSource("x"), false},
		{"youtube matches youtube", channel.NewYoutubeSource("v1"), channel.NewYoutubeSource("v1"), true},
		{"jellyfin matches jellyfin", channel.NewJellyfinSource("j1"), channel.NewJellyfinSource("j1"), true},
		{"youtube vs jellyfin same id", channel.NewYoutubeSource("id"), channel.NewJellyfinSource("id"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Equal(tt.b))
		})
	}
}

func TestSourceIsZero(t *testing.T) {
	tests := []struct {
		name string
		s    channel.Source
		want bool
	}{
		{"zero value struct", channel.Source{}, true},
		{"test source with id", channel.NewTestSource("x"), false},
		{"youtube source with id", channel.NewYoutubeSource("v"), false},
		{"jellyfin source with id", channel.NewJellyfinSource("j"), false},
		// only id matters — empty id is zero regardless of kind
		{"test source empty id", channel.NewTestSource(""), true},
		{"youtube source empty id", channel.NewYoutubeSource(""), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.s.IsZero())
		})
	}
}
