package channel

import (
	"encoding/json"
	"time"
)

// Duration wraps time.Duration to support JSON marshal/unmarshal as a Go
// duration string (e.g. "3m33s").
type Duration struct{ time.Duration }

// UnmarshalJSON parses a Go duration string (e.g. "3m33s") from JSON.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// MarshalJSON writes the duration as a Go duration string (e.g. "3m33s").
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}
