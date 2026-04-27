package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/integration"
	"go-tv/internal/schedule"
	"go-tv/internal/state"
)

// -------------------------------------------------------------------------
// API types
// -------------------------------------------------------------------------

type stateResponse struct {
	VideoID     string  `json:"video_id"`
	Seconds     float64 `json:"seconds"`
	StopSeconds float64 `json:"stop_seconds"`
}

type nextRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

type progressRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

type jumpRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

// wireSegment is the wire format for a playback segment.
type wireSegment struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds,omitempty"`
}

// scheduleVideo is the wire format for a single video within an item.
type scheduleVideo struct {
	ID            string        `json:"id,omitempty"`
	Title         string        `json:"title,omitempty"`
	Segments      []wireSegment `json:"segments,omitempty"`
	LengthSeconds float64       `json:"length_seconds,omitempty"`
}

// scheduleItem is the wire format for a schedule item (named group of videos).
type scheduleItem struct {
	Name   string          `json:"name"`
	Videos []scheduleVideo `json:"videos"`
}

// -------------------------------------------------------------------------
// Server
// -------------------------------------------------------------------------

type Server struct {
	Schedule     *schedule.Schedule
	SchedulePath string
	State        *state.State
	Manager      *integration.Manager
}

func (s *Server) Route(r chi.Router) {
	r.Get("/edit", s.editHandler)
	r.Get("/api/state", s.stateHandler)
	r.Get("/api/schedule", s.scheduleGetHandler)
	r.Post("/api/schedule", s.schedulePostHandler)
	r.Post("/api/progress", s.progressHandler)
	r.Post("/api/next", s.nextHandler)
	r.Post("/api/jump", s.jumpHandler)
	r.Get("/api/integrations", s.integrationsHandler)
	r.Post("/api/integrations/{name}", s.activateIntegrationHandler)
	r.Handle("/*", http.FileServer(http.Dir("static")))
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) currentState() (stateResponse, bool) {
	videoID, seconds := s.State.Get()
	video, ok := s.Schedule.Find(videoID)
	if !ok {
		return stateResponse{}, false
	}
	return stateResponse{
		VideoID:     video.ID,
		Seconds:     seconds,
		StopSeconds: video.StopSecondsAt(seconds),
	}, true
}

func videoToWire(v schedule.Video) scheduleVideo {
	w := scheduleVideo{
		ID:            v.ID,
		Title:         v.Title,
		LengthSeconds: v.Length.Seconds(),
	}
	for _, seg := range v.Segments {
		ws := wireSegment{}
		if seg.Start != nil {
			ws.StartSeconds = seg.Start.Seconds()
		}
		if seg.End != nil {
			ws.EndSeconds = seg.End.Seconds()
		}
		w.Segments = append(w.Segments, ws)
	}
	return w
}

func wireToVideo(w scheduleVideo) schedule.Video {
	v := schedule.Video{
		ID:     w.ID,
		Title:  w.Title,
		Length: schedule.Duration{Duration: time.Duration(w.LengthSeconds * float64(time.Second)).Truncate(time.Second)},
	}
	for _, ws := range w.Segments {
		seg := schedule.Segment{}
		if ws.StartSeconds > 0 {
			d := schedule.Duration{Duration: time.Duration(ws.StartSeconds * float64(time.Second)).Truncate(time.Second)}
			seg.Start = &d
		}
		if ws.EndSeconds > 0 {
			d := schedule.Duration{Duration: time.Duration(ws.EndSeconds * float64(time.Second)).Truncate(time.Second)}
			seg.End = &d
		}
		v.Segments = append(v.Segments, seg)
	}
	return v
}

func itemToWire(it schedule.Item) scheduleItem {
	videos := make([]scheduleVideo, len(it.Videos))
	for i, v := range it.Videos {
		videos[i] = videoToWire(v)
	}
	return scheduleItem{Name: it.Name, Videos: videos}
}

func wireToItem(w scheduleItem) schedule.Item {
	videos := make([]schedule.Video, len(w.Videos))
	for i, v := range w.Videos {
		videos[i] = wireToVideo(v)
	}
	return schedule.Item{Name: w.Name, Videos: videos}
}

// -------------------------------------------------------------------------
// Handlers
// -------------------------------------------------------------------------

func (s *Server) integrationsHandler(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}
	active := s.Manager.Active()
	names := s.Manager.Names()
	entries := make([]entry, len(names))
	for i, n := range names {
		entries[i] = entry{Name: n, Active: n == active}
	}
	writeJSON(w, map[string]any{"integrations": entries}, http.StatusOK)
}

func (s *Server) activateIntegrationHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.Manager.Activate(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) editHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/edit.html")
}

func (s *Server) stateHandler(w http.ResponseWriter, r *http.Request) {
	resp, ok := s.currentState()
	if !ok {
		http.Error(w, "current video not in schedule", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}

func (s *Server) scheduleGetHandler(w http.ResponseWriter, r *http.Request) {
	its := s.Schedule.AllItems()
	entries := make([]scheduleItem, len(its))
	for i, it := range its {
		entries[i] = itemToWire(it)
	}
	writeJSON(w, map[string]any{"items": entries}, http.StatusOK)
}

func (s *Server) schedulePostHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []scheduleItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Build a lookup of existing videos so we can preserve segments.
	existing := make(map[string]schedule.Video)
	for _, v := range s.Schedule.All() {
		existing[v.ID] = v
	}

	items := make([]schedule.Item, len(body.Items))
	for i, entry := range body.Items {
		videos := make([]schedule.Video, len(entry.Videos))
		for j, wv := range entry.Videos {
			if ev, ok := existing[wv.ID]; ok {
				// Preserve existing video data (segments, length); allow title/name changes.
				ev.Title = wv.Title
				videos[j] = ev
			} else {
				videos[j] = wireToVideo(wv)
			}
		}
		items[i] = schedule.Item{Name: entry.Name, Videos: videos}
	}
	s.Schedule.Update(items)
	if err := s.Schedule.Save(s.SchedulePath); err != nil {
		http.Error(w, "failed to save schedule", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jumpHandler(w http.ResponseWriter, r *http.Request) {
	var req jumpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if _, ok := s.Schedule.Find(req.VideoID); !ok {
		http.Error(w, "video not in schedule", http.StatusBadRequest)
		return
	}
	s.State.Jump(req.VideoID, req.Seconds)
	resp, ok := s.currentState()
	if !ok {
		http.Error(w, "current video not in schedule", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}

func (s *Server) progressHandler(w http.ResponseWriter, r *http.Request) {
	var req progressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.State.SetPosition(req.VideoID, req.Seconds)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) nextHandler(w http.ResponseWriter, r *http.Request) {
	var req nextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	pos := time.Duration(req.Seconds * float64(time.Second))
	frag, ok := s.Schedule.Next(req.VideoID, pos)
	if !ok {
		http.Error(w, "no next fragment found", http.StatusInternalServerError)
		return
	}
	s.State.Advance(req.VideoID, frag.ID, frag.Start.Seconds())
	resp, ok := s.currentState()
	if !ok {
		http.Error(w, "current video not in schedule", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}
