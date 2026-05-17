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
	"go-tv/internal/store"
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

	serFiles, err := store.LoadSeriesDir(cfg.SeriesDir)
	if err != nil {
		logger.Error("failed to load series", err)
		os.Exit(1)
	}
	series := make([]*channel.Series, len(serFiles))
	serPaths := make(map[string]string, len(serFiles))
	for i, sf := range serFiles {
		series[i] = sf.Series
		serPaths[sf.Series.ID()] = sf.Path
	}
	schedule := channel.NewSchedule(series...)
	currentState := store.LoadState(cfg.StatePath, schedule)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	saveSeries := func(id string, s *channel.Series) error {
		return store.SaveSeries(serPaths[id], s)
	}
	saveState := func() error {
		return store.SaveState(cfg.StatePath, currentState)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	ch := channel.NewChannel(schedule, currentState, saveSeries, saveState)
	api.OpenChannel(r, ch, cfg.Player, cfg.Jellyfin, logger)

	store.AutoSave(ctx, cfg.StatePath, currentState, 10*time.Minute)

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
	if err := store.SaveState(cfg.StatePath, ch.State()); err != nil {
		logger.Error("failed to save state", err)
		os.Exit(1)
	}
}
