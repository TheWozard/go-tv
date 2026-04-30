package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/config"
	"go-tv/internal/ui/components"
)

func OpenChannel(r chi.Router, ch *channel.Channel, player config.Player, jellyfin config.Jellyfin) {
	s := &Server{channel: ch, player: player, jellyfin: jellyfin}
	s.Route(r)
}

type Server struct {
	channel  *channel.Channel
	player   config.Player
	jellyfin config.Jellyfin
}

func (s *Server) Route(r chi.Router) {
	player := &PlayerHandler{channel: s.channel, jellyfin: s.jellyfin}
	editor := &EditorHandler{channel: s.channel, jellyfin: s.jellyfin}

	r.Get("/", s.playerHandler)
	r.Get("/edit", s.editHandler)
	r.Route("/api", func(r chi.Router) {
		player.Mount(r)
		editor.Mount(r)
		if s.jellyfin.Proxy {
			stream := &StreamHandler{jellyfin: s.jellyfin, client: s.jellyfin.HTTPClient()}
			stream.Mount(r)
		}
		r.Post("/schedule/reorder", s.scheduleReorderHandler)
	})
}

func (s *Server) playerHandler(w http.ResponseWriter, r *http.Request) {
	source, position, stopAt, _ := s.channel.CurrentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.Player(source, position, stopAt, s.player, s.jellyfin).Render(r.Context(), w)
}

func (s *Server) editHandler(w http.ResponseWriter, r *http.Request) {
	source, position, _, _ := s.channel.CurrentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.Editor(s.channel.Playlists(), source, position, s.jellyfin.URL).Render(r.Context(), w)
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

// sourceFromForm parses source_kind and source_id form values into a Source.
func sourceFromForm(r *http.Request) (channel.Source, bool) {
	kind := channel.SourceKind(r.FormValue("source_kind"))
	id := r.FormValue("source_id")
	return channel.NewValidatedSource(kind, id)
}

// parseDuration parses a seconds string from a form value into a time.Duration.
func parseDuration(r *http.Request, key string) time.Duration {
	f, _ := strconv.ParseFloat(r.FormValue(key), 64)
	return time.Duration(f * float64(time.Second)).Truncate(time.Second)
}
