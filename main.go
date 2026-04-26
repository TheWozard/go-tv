package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"go-tv/internal/config"
	"go-tv/internal/integration"
	"go-tv/internal/player/homeassistant"
	"go-tv/internal/schedule"
	"go-tv/internal/server"
	"go-tv/internal/state"
)

func setupHandlers(r chi.Router, cfg *config.Config, sched *schedule.Schedule, st *state.State, mgr *integration.Manager) {
	r.Use(middleware.Recoverer)

	s := &server.Server{
		Schedule:     sched,
		SchedulePath: cfg.SchedulePath,
		State:        st,
		Manager:      mgr,
	}
	s.Route(r)
}

func main() {
	configPath := flag.String("config", "./config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	cfg.ApplyEnvOverrides()

	schedule, err := schedule.Load(cfg.SchedulePath)
	if err != nil {
		log.Fatal(err)
	}

	currentState := state.Load(cfg.StatePath, schedule)
	log.Printf("starting with state: %s", currentState.String())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr := integration.NewManager(ctx, schedule, currentState)
	for _, integ := range cfg.Integrations {
		p := homeassistant.New(homeassistant.Config{
			URL:       integ.URL,
			Token:     integ.Token,
			EntityID:  integ.EntityID,
			MediaType: integ.MediaType,
		})
		mgr.Register(integ.Name, p)
		log.Printf("registered integration %q (%s)", integ.Name, integ.EntityID)
	}

	r := chi.NewRouter()
	setupHandlers(r, cfg, schedule, currentState, mgr)

	var startErr error
	if cfg.Tailscale.Enabled() {
		startErr = server.StartTailscale(ctx, r, cfg.Tailscale.Hostname, cfg.Tailscale.Dir, cfg.Tailscale.Port)
	} else {
		startErr = server.StartHTTP(ctx, r, cfg.Port)
	}
	if startErr != nil {
		log.Fatal(startErr)
	}

	log.Println("shutting down…")
	if err := currentState.Save(cfg.StatePath); err != nil {
		log.Printf("failed to save state: %v", err)
	} else {
		log.Println("state saved to", cfg.StatePath)
	}
}
