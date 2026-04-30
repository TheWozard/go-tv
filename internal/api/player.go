package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/config"
	"go-tv/internal/ui/components"
)

// PlayerHandler serves HTMX endpoints used by the player page.
type PlayerHandler struct {
	channel  *channel.Channel
	jellyfin config.Jellyfin
}

func (h *PlayerHandler) Mount(r chi.Router) {
	r.Post("/progress", h.progressHandler)
	r.Post("/next", h.nextHandler)
}

func (h *PlayerHandler) progressHandler(w http.ResponseWriter, r *http.Request) {
	source, ok := sourceFromForm(r)
	if !ok {
		http.Error(w, "invalid source", http.StatusBadRequest)
		return
	}
	h.channel.Progress(source, parseDuration(r, "seconds"))
	w.WriteHeader(http.StatusNoContent)
}

func (h *PlayerHandler) nextHandler(w http.ResponseWriter, r *http.Request) {
	source, ok := sourceFromForm(r)
	if !ok {
		http.Error(w, "invalid source", http.StatusBadRequest)
		return
	}
	if err := h.channel.Next(source, parseDuration(r, "seconds")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newSource, newPosition, newStop, _ := h.channel.CurrentState()
	streamURL := h.jellyfin.StreamURL(newSource.ID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.PlayerState(newSource, newPosition, newStop, streamURL).Render(r.Context(), w)
}
