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

// scheduleItem is the wire format for a single video in the schedule API.
// Seconds values are used instead of duration strings for easy JS consumption.
type scheduleItem struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	StartSeconds  float64 `json:"start_seconds"`
	StopSeconds   float64 `json:"stop_seconds"`
	LengthSeconds float64 `json:"length_seconds"`
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

func videoToItem(v schedule.Video) scheduleItem {
	item := scheduleItem{
		ID:            v.ID,
		Title:         v.Title,
		StartSeconds:  v.StartSeconds(),
		StopSeconds:   v.Stop.Seconds(),
		LengthSeconds: v.Length.Seconds(),
	}
	// If stop is unset, fall back to length so the slider is usable.
	if item.StopSeconds == 0 {
		item.StopSeconds = item.LengthSeconds
	}
	return item
}

func itemToVideo(item scheduleItem) schedule.Video {
	v := schedule.Video{
		ID:     item.ID,
		Title:  item.Title,
		Stop:   schedule.Duration{Duration: time.Duration(item.StopSeconds * float64(time.Second)).Truncate(time.Second)},
		Length: schedule.Duration{Duration: time.Duration(item.LengthSeconds * float64(time.Second)).Truncate(time.Second)},
	}
	if item.StartSeconds > 0 {
		d := schedule.Duration{Duration: time.Duration(item.StartSeconds * float64(time.Second)).Truncate(time.Second)}
		v.Start = &d
	}
	return v
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
		videos := sched.All()
		items := make([]scheduleItem, len(videos))
		for i, v := range videos {
			items[i] = videoToItem(v)
		}
		writeJSON(w, map[string]any{"videos": items}, http.StatusOK)
	}
}

func schedulePostHandler(sched *schedule.Schedule, schedPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Videos []scheduleItem `json:"videos"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		videos := make([]schedule.Video, len(body.Videos))
		for i, item := range body.Videos {
			videos[i] = itemToVideo(item)
		}
		sched.Update(videos)
		if err := sched.Save(schedPath); err != nil {
			http.Error(w, "failed to save schedule", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
