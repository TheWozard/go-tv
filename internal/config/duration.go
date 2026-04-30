package config

import (
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from YAML strings like "10s" or "1m30s".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) Duration() time.Duration { return time.Duration(d) }
