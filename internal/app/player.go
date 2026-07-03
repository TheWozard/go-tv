package app

import "time"

type Player struct {
	SkipInterval    time.Duration `yaml:"skip_interval" env-default:"10s"`
	ProgressRate    time.Duration `yaml:"progress_rate" env-default:"10s"`
	AdvanceRetry    time.Duration `yaml:"advance_retry"`
	ResyncThreshold time.Duration `yaml:"resync_threshold" env-default:"60s"`
}
