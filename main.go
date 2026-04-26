package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go-tv/internal/schedule"
	"go-tv/internal/state"
)

const stateFile = "state.json"

type stateResponse struct {
	VideoID   string `json:"video_id"`
	StartedAt int64  `json:"started_at"` // Unix milliseconds
}

type nextRequest struct {
	VideoID string `json:"video_id"`
}

func writeJSON(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func currentState(sched *schedule.Schedule, st *state.State) (stateResponse, bool) {
	videoID, startedAt := st.Get()
	video, ok := sched.Find(videoID)
	if !ok {
		return stateResponse{}, false
	}
	return stateResponse{
		VideoID:   video.ID,
		StartedAt: startedAt.UnixMilli(),
	}, true
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

func nextHandler(sched *schedule.Schedule, st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req nextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		next, err := sched.Next(req.VideoID)
		if err != nil {
			http.Error(w, "end of schedule", http.StatusNotFound)
			return
		}

		// First caller with this video_id wins; others are no-ops.
		st.Advance(req.VideoID, next.ID)

		resp, ok := currentState(sched, st)
		if !ok {
			http.Error(w, "current video not in schedule", http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp, http.StatusOK)
	}
}

func main() {
	sched, err := schedule.Load("schedule.json")
	if err != nil {
		log.Fatal(err)
	}

	st, err := state.Load(stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("could not load saved state: %v — starting from beginning", err)
		}
		first := sched.First()
		if first == nil {
			log.Fatal("schedule is empty")
		}
		st = state.New(first.ID)
	} else {
		videoID, _ := st.Get()
		if _, ok := sched.Find(videoID); !ok {
			log.Printf("saved video %q not in schedule — starting from beginning", videoID)
			st = state.New(sched.First().ID)
		} else {
			log.Printf("resuming from saved state: %s", videoID)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("static")))
	mux.HandleFunc("/api/state", stateHandler(sched, st))
	mux.HandleFunc("/api/next", nextHandler(sched, st))

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		log.Println("Listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("Shutting down…")
	if err := st.Save(stateFile); err != nil {
		log.Printf("failed to save state: %v", err)
	} else {
		log.Println("state saved to", stateFile)
	}
	srv.Shutdown(context.Background())
}
