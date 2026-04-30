package config

import (
	"go-tv/internal/log"
	"log/slog"
	"os"
)

type Logger struct {
	Level string `yaml:"level"` // debug, info, warn, error (default: info)
}

// New creates a slog.Logger from the config, writing text to stderr.
func (l Logger) New() *log.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(l.Level)); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	return &log.Logger{Logger: logger}
}
