package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/ui/components"
)

// PlayerHandler serves HTMX endpoints used by the player page.
type PlayerHandler struct {
	channel *channel.Channel
}

func (h *PlayerHandler) Mount(r chi.Router) {
	r.Post("/progress", h.progressHandler)
	r.Post("/next", h.nextHandler)
}

func (h *PlayerHandler) progressHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.FormValue("video_id")
	seconds, _ := strconv.ParseFloat(r.FormValue("seconds"), 64)
	if videoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	h.channel.Progress(videoID, seconds)
	w.WriteHeader(http.StatusNoContent)
}

func (h *PlayerHandler) nextHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.FormValue("video_id")
	seconds, _ := strconv.ParseFloat(r.FormValue("seconds"), 64)
	if videoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.channel.Next(videoID, seconds); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newVideoID, newSeconds, newStop, _ := h.channel.CurrentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.PlayerState(newVideoID, newSeconds, newStop).Render(r.Context(), w)
}
