package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"go-tv/internal/api"
	"go-tv/internal/channel"
	"go-tv/internal/config"
)

//go:embed static
var staticFiles embed.FS

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
	ch := channel.NewChannel(schedule, currentState)
	api.OpenChannel(r, ch)

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("failed to prepare static FS: %v", err)
	}
	r.Handle("/*", http.FileServer(http.FS(sub)))

	if err = cfg.GetServerListener().Listen(ctx, r); err != nil {
		log.Fatal(err)
	}

	log.Println("shutting down")
	if err := ch.SaveState(); err != nil {
		log.Printf("failed to save state: %v", err)
	}
}
