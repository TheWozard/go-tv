package store_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-tv/internal/store"
)

func TestDuration_MarshalJSON(t *testing.T) {
	d := store.Duration{Duration: 3*time.Minute + 33*time.Second}
	data, err := json.Marshal(d)
	require.NoError(t, err)
	assert.JSONEq(t, `"3m33s"`, string(data))
}

func TestDuration_UnmarshalJSON(t *testing.T) {
	var d store.Duration
	err := json.Unmarshal([]byte(`"1h2m3s"`), &d)
	require.NoError(t, err)
	assert.Equal(t, time.Hour+2*time.Minute+3*time.Second, d.Duration)
}

func TestDuration_UnmarshalJSON_Invalid(t *testing.T) {
	var d store.Duration
	assert.Error(t, json.Unmarshal([]byte(`"not-a-duration"`), &d))
	assert.Error(t, json.Unmarshal([]byte(`123`), &d))
}

func TestDuration_RoundTrip(t *testing.T) {
	original := store.Duration{Duration: 45 * time.Second}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded store.Duration
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Duration, decoded.Duration)
}

func TestDuration_JSONStructTags(t *testing.T) {
	original := struct {
		A store.Duration
		C store.Duration `json:",omitempty"`
		B store.Duration `json:",omitzero"`
	}{}

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.JSONEq(t, `{"A": "0s", "C": "0s"}`, string(data))
}
