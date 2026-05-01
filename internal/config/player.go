package config

type Player struct {
	SkipInterval  Duration `yaml:"skip_interval"`
	ProgressRate  Duration `yaml:"progress_rate"`
	AdvanceRetry  Duration `yaml:"advance_retry"`
}
