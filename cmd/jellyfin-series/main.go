// jellyfin-series imports a complete TV series from Jellyfin, creating one
// channel Season per Jellyfin season with all episodes populated automatically.
//
// Usage:
//
//	go run ./cmd/jellyfin-series [flags] <search query>
//
// Flags:
//
//	-config  path to config file (default: config.yaml)
//	-d       path to series directory (default: ./series)
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-tv/internal/app"
	"go-tv/internal/channel"
	"go-tv/internal/store"

	"github.com/TheWozard/go-yaml-config"
)

type jfItem struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	RunTimeTicks *int64 `json:"RunTimeTicks"`
	IndexNumber  *int   `json:"IndexNumber"`
}

func (item jfItem) duration() time.Duration {
	if item.RunTimeTicks == nil || *item.RunTimeTicks == 0 {
		return 0
	}
	return time.Duration(*item.RunTimeTicks * 100).Truncate(time.Second)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	seriesDir := flag.String("d", "./series", "path to series directory")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: jellyfin-series [flags] <search query>")
		os.Exit(1)
	}
	query := strings.Join(flag.Args(), " ")

	cfg, err := config.Load[app.Config](*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}
	if cfg.Jellyfin.URL == "" {
		slog.Error("jellyfin.url not set in config")
		os.Exit(1)
	}

	shows := searchSeries(cfg.Jellyfin, query)
	if len(shows) == 0 {
		fmt.Println("no series found")
		return
	}

	var show jfItem
	if len(shows) == 1 {
		show = shows[0]
		fmt.Printf("found: %s\n", show.Name)
	} else {
		for i, s := range shows {
			fmt.Printf("  %2d. %s\n", i+1, s.Name)
		}
		idx := promptOne(len(shows))
		if idx < 0 {
			fmt.Println("nothing selected")
			return
		}
		show = shows[idx]
	}

	fmt.Printf("fetching seasons for %q…\n", show.Name)
	jfSeasons := fetchSeasons(cfg.Jellyfin, show.ID)
	if len(jfSeasons) == 0 {
		fmt.Println("no seasons found")
		return
	}

	channelSeasons := make([]channel.Season, 0, len(jfSeasons))
	for _, s := range jfSeasons {
		fmt.Printf("  fetching episodes for %s…\n", s.Name)
		eps := fetchEpisodes(cfg.Jellyfin, show.ID, s.ID)
		channelEps := make([]channel.Episode, 0, len(eps))
		for _, ep := range eps {
			ce := channel.NewEpisode(channel.NewJellyfinSource(ep.ID), ep.duration()).WithTitle(ep.Name)
			channelEps = append(channelEps, ce)
		}
		channelSeasons = append(channelSeasons, channel.NewSeason(s.Name, channelEps...))
	}

	if err := os.MkdirAll(*seriesDir, 0755); err != nil {
		slog.Error("failed to create series dir", "err", err)
		os.Exit(1)
	}

	path := filepath.Join(*seriesDir, slugify(show.Name)+".json")
	ser := channel.NewSeries(show.Name, channel.LoopMode, channelSeasons...)
	if err := store.SaveSeries(path, ser); err != nil {
		slog.Error("failed to save series", "err", err)
		os.Exit(1)
	}

	totalEps := 0
	for _, s := range channelSeasons {
		totalEps += len(s.Episodes)
	}
	fmt.Printf("created %q — %d season(s), %d episode(s) → %s\n",
		show.Name, len(channelSeasons), totalEps, path)
}

func apiGet(jf app.Jellyfin, rawURL string, dest any) {
	resp, err := jf.HTTPClient().Get(rawURL)
	if err != nil {
		slog.Error("request failed", "url", rawURL, "err", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		slog.Error("unexpected status", "url", rawURL, "status", resp.Status)
		os.Exit(1)
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		slog.Error("parsing response", "err", err)
		os.Exit(1)
	}
}

func searchSeries(jf app.Jellyfin, query string) []jfItem {
	u, _ := url.Parse(jf.URL + "/Items")
	q := u.Query()
	q.Set("searchTerm", query)
	q.Set("IncludeItemTypes", "Series")
	q.Set("Recursive", "true")
	q.Set("Limit", "20")
	q.Set("Fields", "Name")
	q.Set("api_key", jf.APIKey)
	u.RawQuery = q.Encode()

	var result struct{ Items []jfItem }
	apiGet(jf, u.String(), &result)
	return result.Items
}

func fetchSeasons(jf app.Jellyfin, seriesID string) []jfItem {
	u, _ := url.Parse(jf.URL + "/Shows/" + seriesID + "/Seasons")
	q := u.Query()
	q.Set("Fields", "Name,IndexNumber")
	q.Set("api_key", jf.APIKey)
	u.RawQuery = q.Encode()

	var result struct{ Items []jfItem }
	apiGet(jf, u.String(), &result)
	return result.Items
}

func fetchEpisodes(jf app.Jellyfin, seriesID, seasonID string) []jfItem {
	u, _ := url.Parse(jf.URL + "/Shows/" + seriesID + "/Episodes")
	q := u.Query()
	q.Set("seasonId", seasonID)
	q.Set("Fields", "RunTimeTicks,IndexNumber,Name")
	q.Set("api_key", jf.APIKey)
	u.RawQuery = q.Encode()

	var result struct{ Items []jfItem }
	apiGet(jf, u.String(), &result)
	return result.Items
}

func promptOne(total int) int {
	fmt.Printf("\nselect [1-%d]: ", total)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || n < 1 || n > total {
		return -1
	}
	return n - 1
}
