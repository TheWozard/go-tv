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

	"go-tv/internal/schedule"
)

// ytInfo holds the fields we use from yt-dlp's JSON output.
type ytInfo struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Duration *float64 `json:"duration"` // seconds; nil when unavailable
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

	sched, err := schedule.Load(*schedPath)
	if os.IsNotExist(err) {
		sched = &schedule.Schedule{}
	} else if err != nil {
		log.Fatal(err)
	}

	for _, e := range entries {
		v := schedule.Video{ID: e.ID, Title: e.Title}
		if e.Duration != nil && *e.Duration > 0 {
			d := schedule.Duration{Duration: time.Duration(*e.Duration * float64(time.Second)).Truncate(time.Second)}
			v.Stop = d
			v.Length = d
		}
		sched.Videos = append(sched.Videos, v)
		fmt.Printf("+ %-20s  %-50s  stop=%s\n", e.ID, e.Title, v.Stop.Duration.Truncate(time.Second))
	}

	if err := sched.Save(*schedPath); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("appended %d video(s) to %s\n", len(entries), *schedPath)
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
