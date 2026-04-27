package channel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func dur(d time.Duration) *Duration { return &Duration{d} }

func sec(n int) time.Duration { return time.Duration(n) * time.Second }

func testVideo(segments ...Segment) Video {
	return Video{
		Source:   NewTestSource("v1"),
		Title:    "test",
		Segments: segments,
		Length:   Duration{10 * time.Minute},
	}
}

// Segment helpers

func TestSegment_StartDuration(t *testing.T) {
	assert.Equal(t, time.Duration(0), Segment{}.StartDuration())
	assert.Equal(t, 5*time.Second, Segment{Start: dur(5 * time.Second)}.StartDuration())
}

func TestSegment_EndDuration(t *testing.T) {
	def := 10 * time.Minute
	assert.Equal(t, def, Segment{}.EndDuration(def))
	assert.Equal(t, 30*time.Second, Segment{End: dur(30 * time.Second)}.EndDuration(def))
}

// Video.Current

func TestVideo_Current_NoSegments(t *testing.T) {
	v := testVideo()

	frag, ok := v.Current(0)
	assert.True(t, ok)
	assert.Equal(t, v.Source, frag.Source)
	assert.Equal(t, time.Duration(0), frag.Start)
	assert.Equal(t, v.Length.Duration, frag.End)

	_, ok = v.Current(v.Length.Duration)
	assert.False(t, ok, "at or past length should return false")
}

func TestVideo_Current_WithSegments(t *testing.T) {
	v := testVideo(
		Segment{End: dur(2 * time.Minute)},   // start=0, end=2:00
		Segment{Start: dur(5 * time.Minute)}, // start=5:00, end=10:00
	)

	frag, ok := v.Current(sec(30))
	assert.True(t, ok)
	assert.Equal(t, time.Duration(0), frag.Start)
	assert.Equal(t, 2*time.Minute, frag.End)

	// Position past first segment still returns it
	frag, ok = v.Current(6 * time.Minute)
	assert.True(t, ok)
	assert.Equal(t, time.Duration(0), frag.Start)
}

func TestVideo_Current_PastAllSegments(t *testing.T) {
	v := testVideo(
		Segment{Start: dur(5 * time.Minute), End: dur(8 * time.Minute)},
	)
	_, ok := v.Current(sec(30))
	assert.False(t, ok, "position before all segment starts returns false")
}

// Video.Next

func TestVideo_Next_NoSegments(t *testing.T) {
	v := testVideo()

	frag, ok := v.Next(0)
	assert.True(t, ok)
	assert.Equal(t, time.Duration(0), frag.Start)
	assert.Equal(t, v.Length.Duration, frag.End)

	_, ok = v.Next(sec(1))
	assert.False(t, ok, "non-zero position with no segments returns false")
}

func TestVideo_Next_WithSegments(t *testing.T) {
	v := testVideo(
		Segment{End: dur(2 * time.Minute)},   // start=0
		Segment{Start: dur(5 * time.Minute)}, // start=5m
	)

	frag, ok := v.Next(0)
	assert.True(t, ok)
	assert.Equal(t, 5*time.Minute, frag.Start)

	// Before second segment still returns it
	frag, ok = v.Next(sec(30))
	assert.True(t, ok)
	assert.Equal(t, 5*time.Minute, frag.Start)

	// Past all segments returns false
	_, ok = v.Next(6 * time.Minute)
	assert.False(t, ok)
}

// Video.Clean

func TestVideo_Clean(t *testing.T) {
	length := 10 * time.Minute
	v := Video{
		Length: Duration{length},
		Segments: []Segment{
			{Start: dur(0), End: dur(length)},               // both redundant → dropped
			{Start: dur(0), End: dur(5 * time.Minute)},      // start redundant, end kept
			{Start: dur(3 * time.Minute), End: dur(length)}, // start kept, end redundant
		},
	}
	v.Clean()
	require.Len(t, v.Segments, 2)
	assert.Nil(t, v.Segments[0].Start)
	assert.Equal(t, 5*time.Minute, v.Segments[0].End.Duration)
	assert.Equal(t, 3*time.Minute, v.Segments[1].Start.Duration)
	assert.Nil(t, v.Segments[1].End)
}

// Video JSON round-trip

func TestVideo_JSON_RoundTrip(t *testing.T) {
	v := Video{
		Source: NewTestSource("abc"),
		Title:  "Test Video",
		Length: Duration{3*time.Minute + 30*time.Second},
		Segments: []Segment{
			{Start: dur(10 * time.Second), End: dur(2 * time.Minute)},
		},
	}
	data, err := json.Marshal(v)
	require.NoError(t, err)

	var decoded Video
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.True(t, v.Source.Equal(decoded.Source))
	assert.Equal(t, v.Title, decoded.Title)
	assert.Equal(t, v.Length.Duration, decoded.Length.Duration)
	require.Len(t, decoded.Segments, 1)
	assert.Equal(t, 10*time.Second, decoded.Segments[0].Start.Duration)
	assert.Equal(t, 2*time.Minute, decoded.Segments[0].End.Duration)
}
