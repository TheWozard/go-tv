package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/config"
	"go-tv/internal/log"
	"go-tv/internal/store"
	"go-tv/internal/ui/components"
)

// PlayerHandler serves HTMX endpoints used by the player page.
type PlayerHandler struct {
	channel  *store.ChannelStore
	jellyfin config.Jellyfin
	logger   *log.Logger
}

func (h *PlayerHandler) Mount(r chi.Router) {
	r.Get("/state", h.stateHandler)
	r.Post("/progress", h.progressHandler)
	r.Post("/next", h.nextHandler)
	r.Post("/prev-ep", h.prevEpHandler)
	r.Post("/next-ep", h.nextEpHandler)
}

func (h *PlayerHandler) stateHandler(w http.ResponseWriter, r *http.Request) {
	seg := h.channel.CurrentSegment()
	_, pos := h.channel.State().Get()
	if pos < seg.Clip.Start || pos >= seg.Clip.End {
		pos = seg.Clip.Start
	}
	streamURL := ""
	if seg.Source.Kind == channel.SourceKindJellyfin {
		streamURL = h.jellyfin.StreamURL(seg.Source.ID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PlayerState(seg.Source, pos, seg.Clip.End, streamURL).Render(r.Context(), w); err != nil {
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.channel.Next(source, parseDuration(r, "seconds")); err != nil {
		components.SeriesEndedState().Render(r.Context(), w)
		return
	}
	seg := h.channel.CurrentSegment()
	streamURL := ""
	if seg.Source.Kind == channel.SourceKindJellyfin {
		streamURL = h.jellyfin.StreamURL(seg.Source.ID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PlayerState(seg.Source, seg.Clip.Start, seg.Clip.End, streamURL).Render(r.Context(), w); err != nil {
		h.logger.Error("render player state", err)
	}
}

func (h *PlayerHandler) nextEpHandler(w http.ResponseWriter, r *http.Request) {
	source, ok := sourceFromForm(r)
	if !ok {
		http.Error(w, "invalid source", http.StatusBadRequest)
		return
	}
	_ = h.channel.NextEpisode(source) // if no next episode, current segment is re-rendered from its start
	seg := h.channel.CurrentSegment()
	streamURL := ""
	if seg.Source.Kind == channel.SourceKindJellyfin {
		streamURL = h.jellyfin.StreamURL(seg.Source.ID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PlayerState(seg.Source, seg.Clip.Start, seg.Clip.End, streamURL).Render(r.Context(), w); err != nil {
		h.logger.Error("render player state", err)
	}
}

func (h *PlayerHandler) prevEpHandler(w http.ResponseWriter, r *http.Request) {
	source, ok := sourceFromForm(r)
	if !ok {
		http.Error(w, "invalid source", http.StatusBadRequest)
		return
	}
	_ = h.channel.PrevEpisode(source) // if no prev episode exists, current segment is re-rendered from its start
	seg := h.channel.CurrentSegment()
	streamURL := ""
	if seg.Source.Kind == channel.SourceKindJellyfin {
		streamURL = h.jellyfin.StreamURL(seg.Source.ID)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PlayerState(seg.Source, seg.Clip.Start, seg.Clip.End, streamURL).Render(r.Context(), w); err != nil {
		h.logger.Error("render player state", err)
	}
}
