package channel

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSource(t *testing.T) {
	s := NewSource("abc123")
	assert.Equal(t, SourceKindYoutube, s.Kind)
	assert.Equal(t, "abc123", s.ID)
}

func TestSource_Equal(t *testing.T) {
	a := Source{Kind: SourceKindYoutube, ID: "abc"}
	b := Source{Kind: SourceKindYoutube, ID: "abc"}
	c := Source{Kind: SourceKindYoutube, ID: "xyz"}
	d := Source{Kind: "vimeo", ID: "abc"}

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c), "different ID")
	assert.False(t, a.Equal(d), "different kind")
}

func TestSource_Clean(t *testing.T) {
	s := Source{ID: "abc"}
	assert.Empty(t, s.Kind)
	s.Clean()
	assert.Equal(t, SourceKindYoutube, s.Kind)

	// Already set kind should not change.
	s2 := Source{Kind: "vimeo", ID: "abc"}
	s2.Clean()
	assert.Equal(t, SourceKind("vimeo"), s2.Kind)
}

func TestSource_UnmarshalJSON_Legacy(t *testing.T) {
	var s Source
	err := json.Unmarshal([]byte(`"abc123"`), &s)
	require.NoError(t, err)
	assert.Equal(t, "abc123", s.ID)
	assert.Equal(t, SourceKindYoutube, s.Kind, "legacy format should default to youtube")
}

func TestSource_UnmarshalJSON_Object(t *testing.T) {
	var s Source
	err := json.Unmarshal([]byte(`{"kind":"youtube","id":"xyz789"}`), &s)
	require.NoError(t, err)
	assert.Equal(t, "xyz789", s.ID)
	assert.Equal(t, SourceKindYoutube, s.Kind)
}

func TestSource_UnmarshalJSON_RoundTrip(t *testing.T) {
	original := NewSource("roundtrip")
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Source
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, original.Equal(decoded))
}
