package channel_test

import (
	"testing"
	"time"

	"go-tv/internal/channel"

	"github.com/stretchr/testify/assert"
)

func TestClipCompare(t *testing.T) {
	tests := []struct {
		name     string
		a, b     channel.Clip
		wantSign int // -1, 0, or 1
	}{
		{"equal starts", channel.NewClip(10*time.Second, 30*time.Second), channel.NewClip(10*time.Second, 60*time.Second), 0},
		{"a before b", channel.NewClip(0, 30*time.Second), channel.NewClip(60*time.Second, 90*time.Second), -1},
		{"a after b", channel.NewClip(60*time.Second, 90*time.Second), channel.NewClip(0, 30*time.Second), 1},
		{"both zero start", channel.NewClip(0, 10*time.Second), channel.NewClip(0, 20*time.Second), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Compare(tt.b)
			switch tt.wantSign {
			case -1:
				assert.Less(t, got, 0)
			case 1:
				assert.Greater(t, got, 0)
			default:
				assert.Equal(t, 0, got)
			}
		})
	}
}

func TestClipWindow(t *testing.T) {
	tests := []struct {
		name       string
		start, end time.Duration
	}{
		{"zero", 0, 0},
		{"non-zero", 10 * time.Second, 60 * time.Second},
		{"both equal", 30 * time.Second, 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := channel.NewClip(tt.start, tt.end)
			s, e := c.Window()
			assert.Equal(t, tt.start, s)
			assert.Equal(t, tt.end, e)
		})
	}
}
