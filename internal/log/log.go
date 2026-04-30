package log

import "log/slog"

// ErrKey is the standard slog attribute key for errors.
const ErrKey = "err"

type Logger struct {
	*slog.Logger
}

func (l *Logger) Error(msg string, err error, args ...any) {
	l.Logger.Error(msg, append([]any{ErrKey, err}, args)...)
}

func (l *Logger) WarnOfError(msg string, err error, args ...any) {
	l.Logger.Warn(msg, append([]any{ErrKey, err}, args)...)
}
