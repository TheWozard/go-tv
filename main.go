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

	"go-tv/internal/api"
	"go-tv/internal/channel"
	"go-tv/internal/config"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	schedule, err := channel.LoadSchedule(cfg.SchedulePath)
	if err != nil {
		log.Fatalf("failed to load schedule: %v", err)
	}
	currentState := channel.LoadState(cfg.StatePath, schedule)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	api.OpenChannel(r, schedule, currentState)

	if err = cfg.GetServerListener().Listen(ctx, r); err != nil {
		log.Fatal(err)
	}

	log.Println("shutting down")
	if err := currentState.Save(); err != nil {
		log.Printf("failed to save state: %v", err)
	}
}
