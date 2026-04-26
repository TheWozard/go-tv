package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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
	VideoID string `json:"video_id"`
}

type progressRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

type jumpRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

// scheduleVideo is the wire format for a single video within an item.
type scheduleVideo struct {
	ID            string  `json:"id,omitempty"`
	Title         string  `json:"title,omitempty"`
	StartSeconds  float64 `json:"start_seconds,omitempty"`
	StopSeconds   float64 `json:"stop_seconds,omitempty"`
	LengthSeconds float64 `json:"length_seconds,omitempty"`
}

// scheduleItem is the wire format for a schedule item (named group of videos).
type scheduleItem struct {
	Name   string          `json:"name"`
	Videos []scheduleVideo `json:"videos"`
}

// -------------------------------------------------------------------------
// Mount
// -------------------------------------------------------------------------

func Mount(r chi.Router, sched *schedule.Schedule, schedPath string, st *state.State) {
	r.Get("/edit", editHandler())
	r.Get("/api/state", stateHandler(sched, st))
	r.Get("/api/schedule", scheduleGetHandler(sched))
	r.Post("/api/schedule", schedulePostHandler(sched, schedPath))
	r.Post("/api/progress", progressHandler(st))
	r.Post("/api/next", nextHandler(sched, st))
	r.Post("/api/jump", jumpHandler(sched, st))
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

func currentState(sched *schedule.Schedule, st *state.State) (stateResponse, bool) {
	videoID, seconds := st.Get()
	video, ok := sched.Find(videoID)
	if !ok {
		return stateResponse{}, false
	}
	return stateResponse{
		VideoID:     video.ID,
		Seconds:     seconds,
		StopSeconds: video.Stop.Seconds(),
	}, true
}

func videoToWire(v schedule.Video) scheduleVideo {
	w := scheduleVideo{
		ID:            v.ID,
		Title:         v.Title,
		StartSeconds:  v.StartSeconds(),
		StopSeconds:   v.Stop.Seconds(),
		LengthSeconds: v.Length.Seconds(),
	}
	if w.StopSeconds == 0 {
		w.StopSeconds = w.LengthSeconds
	}
	return w
}

func wireToVideo(w scheduleVideo) schedule.Video {
	v := schedule.Video{
		ID:     w.ID,
		Title:  w.Title,
		Stop:   schedule.Duration{Duration: time.Duration(w.StopSeconds * float64(time.Second)).Truncate(time.Second)},
		Length: schedule.Duration{Duration: time.Duration(w.LengthSeconds * float64(time.Second)).Truncate(time.Second)},
	}
	if w.StartSeconds > 0 {
		d := schedule.Duration{Duration: time.Duration(w.StartSeconds * float64(time.Second)).Truncate(time.Second)}
		v.Start = &d
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

func editHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/edit.html")
	}
}

func stateHandler(sched *schedule.Schedule, st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, ok := currentState(sched, st)
		if !ok {
			http.Error(w, "current video not in schedule", http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp, http.StatusOK)
	}
}

func scheduleGetHandler(sched *schedule.Schedule) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		its := sched.AllItems()
		entries := make([]scheduleItem, len(its))
		for i, it := range its {
			entries[i] = itemToWire(it)
		}
		writeJSON(w, map[string]any{"items": entries}, http.StatusOK)
	}
}

func schedulePostHandler(sched *schedule.Schedule, schedPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Items []scheduleItem `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		items := make([]schedule.Item, len(body.Items))
		for i, entry := range body.Items {
			items[i] = wireToItem(entry)
		}
		sched.Update(items)
		if err := sched.Save(schedPath); err != nil {
			http.Error(w, "failed to save schedule", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func jumpHandler(sched *schedule.Schedule, st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jumpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if _, ok := sched.Find(req.VideoID); !ok {
			http.Error(w, "video not in schedule", http.StatusBadRequest)
			return
		}
		st.Jump(req.VideoID, req.Seconds)
		resp, ok := currentState(sched, st)
		if !ok {
			http.Error(w, "current video not in schedule", http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp, http.StatusOK)
	}
}

func progressHandler(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req progressRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		st.SetPosition(req.VideoID, req.Seconds)
		w.WriteHeader(http.StatusNoContent)
	}
}

func nextHandler(sched *schedule.Schedule, st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req nextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		next, err := sched.Next(req.VideoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		st.Advance(req.VideoID, next.ID, next.StartSeconds())
		resp, ok := currentState(sched, st)
		if !ok {
			http.Error(w, "current video not in schedule", http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp, http.StatusOK)
	}
}
