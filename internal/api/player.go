package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/config"
	"go-tv/internal/log"
	"go-tv/internal/ui/components"
)

// PlayerHandler serves HTMX endpoints used by the player page.
type PlayerHandler struct {
	channel  *channel.Channel
	jellyfin config.Jellyfin
	logger   *log.Logger
}

func (h *PlayerHandler) Mount(r chi.Router) {
	r.Get("/state", h.stateHandler)
	r.Post("/progress", h.progressHandler)
	r.Post("/next", h.nextHandler)
}

func (h *PlayerHandler) stateHandler(w http.ResponseWriter, r *http.Request) {
	frag := h.channel.CurrentFragment()
	streamURL := ""
	if frag.Source.Kind == channel.SourceKindJellyfin {
		streamURL = h.jellyfin.StreamURL(frag.Source.ID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PlayerState(frag.Source, frag.Start, frag.End, streamURL).Render(r.Context(), w); err != nil {
		h.logger.Error("render player state", err)
	}
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
	frag := h.channel.CurrentFragment()
	streamURL := ""
	if frag.Source.Kind == channel.SourceKindJellyfin {
		streamURL = h.jellyfin.StreamURL(frag.Source.ID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PlayerState(frag.Source, frag.Start, frag.End, streamURL).Render(r.Context(), w); err != nil {
		h.logger.Error("render player state", err)
	}
}
