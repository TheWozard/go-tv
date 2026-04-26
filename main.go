package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/player"
	"go-tv/internal/player/homeassistant"
	"go-tv/internal/schedule"
	"go-tv/internal/server"
	"go-tv/internal/state"
)

func main() {
	scheduleFile := flag.String("schedule", "schedule.json", "path to schedule file")
	stateFile    := flag.String("state", "state.json", "path to state file")
	haURL        := flag.String("ha-url", "", "Home Assistant base URL (e.g. http://homeassistant.local:8123)")
	haToken      := flag.String("ha-token", "", "Home Assistant long-lived access token")
	haEntity     := flag.String("ha-entity", "", "Home Assistant media_player entity ID")
	flag.Parse()

	schedule, err := schedule.Load(*scheduleFile)
	if err != nil {
		log.Fatal(err)
	}

	defaultState := &state.State{VideoID: schedule.First().ID}
	currentState := state.Load(*stateFile, defaultState)
	if _, ok := schedule.Find(currentState.VideoID); !ok {
		currentState = defaultState
	}
	log.Printf("Starting with state: %s", currentState.String())

	r := chi.NewRouter()
	server.Mount(r, schedule, *scheduleFile, currentState)
	srv := &http.Server{Addr: ":8080", Handler: r}
	go func() {
		log.Println("Listening on http://localhost:8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *haURL != "" && *haToken != "" && *haEntity != "" {
		p := homeassistant.New(homeassistant.Config{
			URL:      *haURL,
			Token:    *haToken,
			EntityID: *haEntity,
		})
		go player.Run(ctx, p, schedule, currentState)
		log.Printf("Home Assistant player active: %s", *haEntity)
	}

	<-ctx.Done()

	log.Println("Shutting down…")
	if err := currentState.Save(*stateFile); err != nil {
		log.Printf("failed to save state: %v", err)
	} else {
		log.Println("state saved to", *stateFile)
	}
	srv.Shutdown(context.Background())
}
