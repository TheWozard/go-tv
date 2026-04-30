// clean runs Video.Clean on all videos in the schedule and saves.
//
// Usage:
//
//	go run ./cmd/clean [-s schedule.json]
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"go-tv/internal/channel"
)

func main() {
	schedPath := flag.String("s", "schedule.json", "path to schedule file")
	flag.Parse()

	sched, err := channel.LoadSchedule(*schedPath)
	if err != nil {
		slog.Error("failed to load schedule", "err", err)
		os.Exit(1)
	}

	items := sched.AllItems()
	for i, item := range items {
		for j := range item.Videos {
			items[i].Videos[j].Clean()
		}
	}

	sched.Update(items)
	if err := sched.Save(); err != nil {
		slog.Error("failed to save schedule", "err", err)
		os.Exit(1)
	}
	fmt.Printf("cleaned %s\n", *schedPath)
}
