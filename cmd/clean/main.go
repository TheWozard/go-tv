// clean runs Video.Clean on all videos in the schedule and saves.
//
// Usage:
//
//	go run ./cmd/clean [-s schedule.json]
package main

import (
	"flag"
	"fmt"
	"log"

	"go-tv/internal/schedule"
)

func main() {
	schedPath := flag.String("s", "schedule.json", "path to schedule file")
	flag.Parse()

	sched, err := schedule.Load(*schedPath)
	if err != nil {
		log.Fatal(err)
	}

	items := sched.AllItems()
	for i, item := range items {
		for j := range item.Videos {
			items[i].Videos[j].Clean()
		}
	}

	sched.Update(items)
	if err := sched.Save(*schedPath); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("cleaned %s\n", *schedPath)
}
