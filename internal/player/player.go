package player

import (
	"context"
	"log"
	"time"

	"go-tv/internal/schedule"
	"go-tv/internal/state"
)

// Event is a position update from a Player. Playing=false means the player
// has stopped; the Run loop will advance to the next video. Closing the
// channel has the same effect and is the preferred signal for a clean end.
type Event struct {
	Seconds float64
	Playing bool
}

// Player controls an external media player.
//
// Play starts playback of video at seconds and returns a channel of position
// updates. The channel should be closed when playback ends. Cancelling ctx
// signals the player to stop sending and close the channel.
//
// Implementations may be push-based (writing to the channel from a webhook or
// WebSocket goroutine) or pull-based (writing from an internal polling loop).
type Player interface {
	Play(ctx context.Context, video schedule.Video, seconds float64) (<-chan Event, error)
}

const stateCheckInterval = 2 * time.Second

// Run drives p until ctx is cancelled, keeping st in sync with what p reports
// and advancing sched when the stop point is reached or playback ends.
//
// A background ticker re-checks the shared state every stateCheckInterval so
// that external changes (e.g. a jump via the HTTP API) are picked up quickly
// even between player events.
func Run(ctx context.Context, p Player, sched *schedule.Schedule, st *state.State) {
	var (
		events      <-chan Event
		playCancel  context.CancelFunc
		activeVideo string
	)

	cancel := func() {
		if playCancel != nil {
			playCancel()
			playCancel = nil
		}
		events = nil
		activeVideo = ""
	}

	advance := func() {
		next, err := sched.Next(activeVideo)
		if err != nil {
			log.Printf("player: next: %v", err)
			cancel()
			return
		}
		st.Advance(activeVideo, next.ID, next.StartSeconds())
		cancel()
	}

	startPlaying := func() {
		videoID, seconds := st.Get()
		if videoID == activeVideo {
			return
		}
		cancel()
		video, ok := sched.Find(videoID)
		if !ok {
			return
		}
		playCtx, playCancel2 := context.WithCancel(ctx)
		playCancel = playCancel2
		ch, err := p.Play(playCtx, *video, seconds)
		if err != nil {
			log.Printf("player: play %s: %v", videoID, err)
			playCancel()
			playCancel = nil
			return
		}
		events = ch
		activeVideo = videoID
	}

	ticker := time.NewTicker(stateCheckInterval)
	defer ticker.Stop()

	startPlaying()

	for {
		select {
		case <-ctx.Done():
			cancel()
			return

		case <-ticker.C:
			videoID, _ := st.Get()
			if videoID != activeVideo {
				startPlaying()
			}

		case ev, ok := <-events:
			if !ok || !ev.Playing {
				advance()
				continue
			}
			st.SetPosition(activeVideo, ev.Seconds)
			video, ok := sched.Find(activeVideo)
			if ok {
				if stop := video.Stop.Seconds(); stop > 0 && ev.Seconds >= stop {
					advance()
				}
			}
		}
	}
}
