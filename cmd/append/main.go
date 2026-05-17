// append fetches metadata for a YouTube video or playlist via yt-dlp and
// creates a new series file in the series directory. No video content is downloaded.
//
// Usage:
//
//	go run ./cmd/append [flags] <youtube-url>
//
// Flags:
//
//	-d  path to series directory (default: ./series)
//
// Requires yt-dlp to be installed and available on PATH.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-tv/internal/channel"
	"go-tv/internal/store"
)

// ytInfo holds the fields we use from yt-dlp's JSON output.
type ytInfo struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Duration      *float64 `json:"duration"`       // seconds; nil when unavailable
	PlaylistTitle string   `json:"playlist_title"` // non-empty when part of a playlist
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func main() {
	seriesDir := flag.String("d", "./series", "path to series directory")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: append [flags] <youtube-url>")
		os.Exit(1)
	}

	entries := fetch(flag.Arg(0))
	if len(entries) == 0 {
		slog.Error("no videos found")
		os.Exit(1)
	}

	name := entries[0].PlaylistTitle
	if name == "" {
		name = entries[0].Title
	}

	episodes := make([]channel.Episode, len(entries))
	for i, e := range entries {
		episodes[i] = toEpisode(e)
		fmt.Printf("  + %-20s  %s\n", episodes[i].Source.ID, episodes[i].Title)
	}

	if err := os.MkdirAll(*seriesDir, 0755); err != nil {
		slog.Error("failed to create series dir", "err", err)
		os.Exit(1)
	}

	path := filepath.Join(*seriesDir, slugify(name)+".json")
	season := channel.Season{Name: name, Episodes: episodes}
	ser := channel.NewSeries(name, season)
	if err := store.SaveSeries(path, ser); err != nil {
		slog.Error("failed to save series", "err", err)
		os.Exit(1)
	}
	fmt.Printf("created series %q (%d episode(s)) → %s\n", name, len(episodes), path)
}

func toEpisode(e ytInfo) channel.Episode {
	v := channel.Episode{Source: channel.NewYoutubeSource(e.ID), Title: e.Title}
	if e.Duration != nil && *e.Duration > 0 {
		v.Length = time.Duration(*e.Duration * float64(time.Second)).Truncate(time.Second)
	}
	return v
}

// fetch runs yt-dlp and returns one ytInfo per video.
func fetch(url string) []ytInfo {
	out, err := exec.Command("yt-dlp", "--flat-playlist", "--dump-json", url).Output()
	if err != nil {
		slog.Error("yt-dlp failed", "err", err)
		os.Exit(1)
	}

	var entries []ytInfo
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var info ytInfo
		if err := dec.Decode(&info); err != nil {
			slog.Error("parsing yt-dlp output", "err", err)
			os.Exit(1)
		}
		entries = append(entries, info)
	}
	return entries
}
