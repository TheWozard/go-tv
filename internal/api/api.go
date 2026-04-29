package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/ui/components"
)

func OpenChannel(r chi.Router, ch *channel.Channel) {
	s := &Server{channel: ch}
	s.Route(r)
}

type Server struct {
	channel *channel.Channel
}

func (s *Server) Route(r chi.Router) {
	player := &PlayerHandler{channel: s.channel}
	editor := &EditorHandler{channel: s.channel}

	r.Get("/", s.playerHandler)
	r.Get("/edit", s.editHandler)
	r.Route("/api", func(r chi.Router) {
		player.Mount(r)
		editor.Mount(r)
		r.Post("/schedule/reorder", s.scheduleReorderHandler)
	})
}

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
