package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"go-tv/internal/channel"
	"go-tv/internal/ui"
	"go-tv/internal/ui/components"
)

func OpenChannel(r chi.Router, ch *channel.Channel) {
	s := &Server{
		channel:     ch,
		broadcaster: ui.NewBroadcaster(),
	}
	s.Route(r)
}

type Server struct {
	channel     *channel.Channel
	broadcaster *ui.Broadcaster
}

func (s *Server) Route(r chi.Router) {
	r.Get("/", s.playerHandler)
	r.Get("/edit", s.editHandler)
	r.Route("/sse", func(r chi.Router) {
		r.Get("/state", s.sseStateHandler)
	})
	r.Route("/api", func(r chi.Router) {
		r.Post("/progress", s.progressHandler)
		r.Post("/next", s.nextHandler)
		r.Post("/jump", s.jumpHandler)
		r.Route("/sponsorblock", (&SponsorBlock{Channel: s.channel}).Route)
		r.Route("/schedule", func(r chi.Router) {
			r.Post("/rename", s.scheduleRenameHandler)
			r.Post("/reorder", s.scheduleReorderHandler)
		})
	})
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func (s *Server) broadcastState(r *http.Request) {
	videoID, seconds, stopSeconds, ok := s.channel.CurrentState()
	if !ok {
		return
	}
	var buf bytes.Buffer
	if err := components.PlayerState(videoID, seconds, stopSeconds).Render(r.Context(), &buf); err != nil {
		return
	}
	s.broadcaster.Broadcast(buf.String())
}

// -------------------------------------------------------------------------
// Page handlers
// -------------------------------------------------------------------------

func (s *Server) playerHandler(w http.ResponseWriter, r *http.Request) {
	videoID, seconds, stopSeconds, _ := s.channel.CurrentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.Player(videoID, seconds, stopSeconds).Render(r.Context(), w)
}

func (s *Server) editHandler(w http.ResponseWriter, r *http.Request) {
	videoID, seconds, _, _ := s.channel.CurrentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.Editor(s.channel.Playlists(), videoID, seconds).Render(r.Context(), w)
}

// -------------------------------------------------------------------------
// SSE handler
// -------------------------------------------------------------------------

func (s *Server) sseStateHandler(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	if videoID, seconds, stopSeconds, ok := s.channel.CurrentState(); ok {
		if err := sse.PatchElementTempl(components.PlayerState(videoID, seconds, stopSeconds)); err != nil {
			return
		}
	}

	id, ch := s.broadcaster.Subscribe()
	defer s.broadcaster.Unsubscribe(id)

	for {
		select {
		case html, ok := <-ch:
			if !ok {
				return
			}
			if err := sse.PatchElements(html); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// -------------------------------------------------------------------------
// State handlers
// -------------------------------------------------------------------------

func (s *Server) progressHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VideoID string  `json:"video_id"`
		Seconds float64 `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.channel.Progress(req.VideoID, req.Seconds)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jumpHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VideoID string  `json:"video_id"`
		Seconds float64 `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.channel.Jump(req.VideoID, req.Seconds); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.broadcastState(r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) nextHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VideoID string  `json:"video_id"`
		Seconds float64 `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.channel.Next(req.VideoID, req.Seconds); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.broadcastState(r)
	w.WriteHeader(http.StatusNoContent)
}

// -------------------------------------------------------------------------
// Schedule handlers
// -------------------------------------------------------------------------

func (s *Server) scheduleRenameHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		OldName string `json:"old_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.channel.RenamePlaylist(req.OldName, req.Name); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) scheduleReorderHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			Name     string   `json:"name"`
			VideoIDs []string `json:"video_ids"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	sets := make([]channel.ReorderSet, len(req.Items))
	for i, item := range req.Items {
		sets[i] = channel.ReorderSet{Name: item.Name, VideoIDs: item.VideoIDs}
	}
	if err := s.channel.Reorder(sets); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
