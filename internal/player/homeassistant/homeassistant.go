// Package homeassistant implements player.Player against the Home Assistant
// REST API. Play sends a media_player.play_media service call with a YouTube
// deep-link URL. Position is tracked by polling the entity state and
// estimating elapsed time since the last HA position update.
package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"go-tv/internal/player"
	"go-tv/internal/schedule"
)

const defaultPollInterval = 5 * time.Second

// Config holds the credentials and target entity for a Home Assistant instance.
type Config struct {
	URL       string // e.g. "http://homeassistant.local:8123"
	Token     string // long-lived access token
	EntityID  string // e.g. "media_player.living_room_tv"
	MediaType string // passed to play_media as media_content_type; defaults to "url"
}

// Player implements player.Player using the Home Assistant REST API.
type Player struct {
	cfg          Config
	client       *http.Client
	pollInterval time.Duration
}

// New returns a Player with default settings.
func New(cfg Config) *Player {
	return &Player{
		cfg:          cfg,
		client:       &http.Client{Timeout: 10 * time.Second},
		pollInterval: defaultPollInterval,
	}
}

// Play sends a play_media service call to HA and returns a channel of position
// events. The internal polling goroutine runs until ctx is cancelled or the
// entity reports a terminal state (idle/off/unavailable), at which point the
// channel is closed.
func (p *Player) Play(ctx context.Context, video schedule.Video, seconds float64) (<-chan player.Event, error) {
	if _, err := p.getEntityState(ctx); err != nil {
		return nil, fmt.Errorf("entity %s not reachable: %w", p.cfg.EntityID, err)
	}
	if err := p.sendPlay(ctx, video, seconds); err != nil {
		return nil, err
	}
	ch := make(chan player.Event, 1)
	go p.pollLoop(ctx, ch)
	return ch, nil
}

// sendPlay calls the media_player.play_media HA service.
func (p *Player) sendPlay(ctx context.Context, video schedule.Video, seconds float64) error {
	mediaType := p.cfg.MediaType
	if mediaType == "" {
		mediaType = "video"
	}
	contentID := fmt.Sprintf("youtube://www.youtube.com/watch?v=%s&t=%d", video.ID, int(seconds))
	body, err := json.Marshal(map[string]any{
		"entity_id":          p.cfg.EntityID,
		"media_content_id":   contentID,
		"media_content_type": mediaType,
	})
	if err != nil {
		return err
	}
	log.Printf("ha: play_media %s", body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.URL+"/api/services/media_player/play_media",
		strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HA returned %s: %s", resp.Status, strings.TrimSpace(string(detail)))
	}
	return nil
}

// haState is the subset of the HA entity state we care about.
type haState struct {
	State      string `json:"state"`
	Attributes struct {
		MediaPosition          *float64  `json:"media_position"`
		MediaPositionUpdatedAt time.Time `json:"media_position_updated_at"`
	} `json:"attributes"`
}

func (p *Player) getEntityState(ctx context.Context) (*haState, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.cfg.URL+"/api/states/"+p.cfg.EntityID, nil)
	if err != nil {
		return nil, err
	}
	p.auth(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HA returned %s: %s", resp.Status, strings.TrimSpace(string(detail)))
	}
	var s haState
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// pollLoop polls the HA entity and writes Events to ch until the entity
// reaches a terminal state or ctx is cancelled.
func (p *Player) pollLoop(ctx context.Context, ch chan<- player.Event) {
	defer close(ch)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		s, err := p.getEntityState(ctx)
		if err != nil {
			continue
		}

		switch s.State {
		case "idle", "off", "unavailable":
			// Terminal: signal end of playback by closing the channel.
			return
		case "paused":
			// Not moving; send a non-playing event so the run loop doesn't
			// advance, but don't estimate drift.
			select {
			case ch <- player.Event{Seconds: positionOf(s), Playing: false}:
			default:
			}
		case "playing":
			pos := positionOf(s)
			// Estimate drift since HA last updated the position.
			pos += time.Since(s.Attributes.MediaPositionUpdatedAt).Seconds()
			select {
			case ch <- player.Event{Seconds: pos, Playing: true}:
			default:
			}
		}
	}
}

// positionOf returns the raw media_position from an HA state, or 0.
func positionOf(s *haState) float64 {
	if s.Attributes.MediaPosition == nil {
		return 0
	}
	return *s.Attributes.MediaPosition
}

func (p *Player) auth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
}
