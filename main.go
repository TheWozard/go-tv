package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		config.Logger{Level: "info"}.New().Error("failed to load config", err)
		os.Exit(1)
	}

	logger := cfg.Logger.New()

	schedule, err := channel.LoadSchedule(cfg.SchedulePath)
	if err != nil {
		logger.Error("failed to load schedule", err)
		os.Exit(1)
	}
	currentState := channel.LoadState(cfg.StatePath, schedule)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	ch := channel.NewChannel(schedule, currentState)
	api.OpenChannel(r, ch, cfg.Player, cfg.Jellyfin, logger)

	ch.StartAutoSave(ctx, 10*time.Minute)

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		logger.Error("failed to prepare static FS", err)
		os.Exit(1)
	}
	r.Handle("/*", http.FileServer(http.FS(sub)))

	if err = cfg.GetServerListener().Listen(ctx, r, logger); err != nil {
		logger.Error("server error", err)
		os.Exit(1)
	}

	logger.Info("shutting down")
	if err := ch.SaveState(); err != nil {
		logger.Error("failed to save state", err)
		os.Exit(1)
	}
}
