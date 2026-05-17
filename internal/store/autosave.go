package store

import (
	"context"
	"time"

	"go-tv/internal/channel"
)

// AutoSave saves state to path on interval until ctx is cancelled.
func AutoSave(ctx context.Context, path string, s *channel.State, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = SaveState(path, s)
			case <-ctx.Done():
				return
			}
		}
	}()
}
