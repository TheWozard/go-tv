// append fetches metadata for a YouTube video or playlist via yt-dlp and
// appends the entries to a schedule.json file. No video content is downloaded.
//
// Usage:
//
//	go run ./cmd/append [flags] <youtube-url>
//
// Flags:
//
//	-schedule  path to schedule file (default: schedule.json)
//
// Requires yt-dlp to be installed and available on PATH.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"go-tv/internal/channel"
)

// ytInfo holds the fields we use from yt-dlp's JSON output.
type ytInfo struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Duration      *float64 `json:"duration"`       // seconds; nil when unavailable
	PlaylistTitle string   `json:"playlist_title"` // non-empty when part of a playlist
}

func main() {
	schedPath := flag.String("schedule", "schedule.json", "path to schedule file")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: append [flags] <youtube-url>")
		os.Exit(1)
	}

	entries := fetch(flag.Arg(0))
	if len(entries) == 0 {
		log.Fatal("no videos found")
	}

	sched, err := channel.LoadSchedule(*schedPath)
	if os.IsNotExist(err) {
		sched = channel.NewSchedule(*schedPath)
	} else if err != nil {
		log.Fatal(err)
	}

	name := entries[0].PlaylistTitle
	if name == "" {
		name = entries[0].Title
	}

	videos := make([]channel.Video, len(entries))
	for i, e := range entries {
		videos[i] = toVideo(e)
		fmt.Printf("  + %-20s  %s\n", videos[i].Source.ID, videos[i].Title)
	}

	items := append(sched.AllItems(), channel.Playlist{Name: name, Videos: videos})
	fmt.Printf("added %q (%d video(s))\n", name, len(videos))
	sched.Update(items)

	if err := sched.Save(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("appended to %s\n", *schedPath)
}

func toVideo(e ytInfo) channel.Video {
	v := channel.Video{Source: channel.NewYoutubeSource(e.ID), Title: e.Title}
	if e.Duration != nil && *e.Duration > 0 {
		d := channel.Duration{Duration: time.Duration(*e.Duration * float64(time.Second)).Truncate(time.Second)}
		v.Length = d
	}
	return v
}

// fetch runs yt-dlp and returns one ytInfo per video.
// --flat-playlist handles both single video URLs and playlist URLs without
// downloading any video content.
func fetch(url string) []ytInfo {
	out, err := exec.Command("yt-dlp", "--flat-playlist", "--dump-json", url).Output()
	if err != nil {
		log.Fatalf("yt-dlp: %v", err)
	}

	var entries []ytInfo
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var info ytInfo
		if err := dec.Decode(&info); err != nil {
			log.Fatalf("parsing yt-dlp output: %v", err)
		}
		entries = append(entries, info)
	}
	return entries
}
