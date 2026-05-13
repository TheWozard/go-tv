// clean runs Episode.Clean on all episodes in every series and saves each file.
//
// Usage:
//
//	go run ./cmd/clean [-d ./series]
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"go-tv/internal/channel"
)

func main() {
	seriesDir := flag.String("d", "./series", "path to series directory")
	flag.Parse()

	sched, err := channel.LoadSeriesDir(*seriesDir)
	if err != nil {
		slog.Error("failed to load series dir", "err", err)
		os.Exit(1)
	}

	for _, ser := range sched.AllSeries() {
		seasons := ser.AllSeasons()
		for i, season := range seasons {
			for j := range season.Episodes {
				seasons[i].Episodes[j].Clean()
			}
		}
		ser.UpdateSeasons(seasons)
		if err := ser.Save(); err != nil {
			slog.Error("failed to save series", "name", ser.Name, "err", err)
			os.Exit(1)
		}
		fmt.Printf("cleaned %s\n", ser.Name)
	}
}
