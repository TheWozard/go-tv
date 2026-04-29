package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/client/sponsorblock"
	"go-tv/internal/ui/components"
)

// EditorHandler serves HTMX endpoints used by the editor page.
type EditorHandler struct {
	channel *channel.Channel
}

func (h *EditorHandler) Mount(r chi.Router) {
	r.Post("/jump", h.jumpHandler)
	r.Post("/schedule/rename", h.renameHandler)
	r.Get("/sponsorblock/{videoID}", h.sbGetHandler)
	r.Post("/sponsorblock/{videoID}", h.sbPostHandler)
}

func (h *EditorHandler) jumpHandler(w http.ResponseWriter, r *http.Request) {
	source, ok := sourceFromForm(r)
	if !ok {
		http.Error(w, "invalid source", http.StatusBadRequest)
		return
	}
	if err := h.channel.Jump(source, parseDuration(r, "seconds")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	activeSource, activeAt, _, _ := h.channel.CurrentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.VideoList(h.channel.Playlists(), activeSource, activeAt).Render(r.Context(), w)
}

func (h *EditorHandler) renameHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	oldName := r.FormValue("old_name")
	if name == "" || oldName == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.channel.RenamePlaylist(oldName, name); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EditorHandler) sbGetHandler(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	segs, err := sponsorblock.New().GetSegments(videoID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	sbSegs := make([]components.SBSegment, len(segs))
	for i, seg := range segs {
		sbSegs[i] = components.SBSegment{
			Category:     string(seg.Category),
			StartSeconds: seg.Segment[0],
			EndSeconds:   seg.Segment[1],
			Votes:        seg.Votes,
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.SponsorBlockPanel(videoID, sbSegs).Render(r.Context(), w)
}

func (h *EditorHandler) sbPostHandler(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	cuts := make([]channel.CutRange, 0, len(r.Form["cuts"]))
	for _, s := range r.Form["cuts"] {
		parts := strings.SplitN(s, ",", 2)
		if len(parts) != 2 {
			continue
		}
		start, _ := strconv.ParseFloat(parts[0], 64)
		end, _ := strconv.ParseFloat(parts[1], 64)
		cuts = append(cuts, channel.CutRange{
			Start: time.Duration(start * float64(time.Second)).Truncate(time.Second),
			End:   time.Duration(end * float64(time.Second)).Truncate(time.Second),
		})
	}
	updated, err := h.channel.ApplyCuts(videoID, cuts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.VideoCard(*updated, false, 0).Render(r.Context(), w)
}
