package channel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSource(t *testing.T) {
	s := NewTestSource("abc123")
	assert.Equal(t, SourceKindYoutube, s.Kind)
	assert.Equal(t, "abc123", s.ID)
}

func TestSource_Equal(t *testing.T) {
	a := NewTestSource("abc")
	b := NewTestSource("abc")
	c := NewTestSource("xyz")
	d := NewYoutubeSource("abc")

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c), "different ID")
	assert.False(t, a.Equal(d), "different kind")
}
