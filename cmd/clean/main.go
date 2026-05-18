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

	"go-tv/internal/store"
)

func main() {
	seriesDir := flag.String("d", "./series", "path to series directory")
	flag.Parse()

	serFiles, err := store.LoadSeriesDir(*seriesDir)
	if err != nil {
		slog.Error("failed to load series dir", "err", err)
		os.Exit(1)
	}

	for _, sf := range serFiles {
		ser := sf.Series
		seasons := ser.Seasons
		for i, season := range seasons {
			for j := range season.Episodes {
				seasons[i].Episodes[j] = store.CleanEpisode(seasons[i].Episodes[j])
			}
		}
		ser.Seasons = seasons
		if err := store.SaveSeries(sf.Path, ser); err != nil {
			slog.Error("failed to save series", "name", ser.Name, "err", err)
			os.Exit(1)
		}
		fmt.Printf("cleaned %s\n", ser.Name)
	}
}
