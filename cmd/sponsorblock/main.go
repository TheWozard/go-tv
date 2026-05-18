// sponsorblock fetches segments from SponsorBlock for a video and updates
// the series. Each selected segment splits the video's playback, creating
// gaps where skipped content lives. Segments shorter than -min are discarded.
//
// Usage:
//
//	go run ./cmd/sponsorblock [flags] <video-id>
//
// Flags:
//
//	-d     path to series directory (default: ./series)
//	-min   minimum segment duration to keep (default: 10s)
//	-apply pre-select segments (comma-separated numbers, or 'all')
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-tv/internal/channel"
	"go-tv/internal/client/sponsorblock"
	"go-tv/internal/store"
)

func main() {
	seriesDir := flag.String("d", "./series", "path to series directory")
	minDur := flag.Duration("m", 10*time.Second, "minimum segment duration to keep")
	apply := flag.String("a", "", "segments to apply (comma-separated numbers, or 'all')")
	cats := flag.String("c", "", "categories to fetch (comma-separated; default: all)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: sponsorblock [flags] <video-id>")
		os.Exit(1)
	}
	videoID := flag.Arg(0)

	serFiles, err := store.LoadSeriesDir(*seriesDir)
	if err != nil {
		slog.Error("failed to load series dir", "err", err)
		os.Exit(1)
	}
	series := make([]*channel.Series, len(serFiles))
	serPaths := make(map[string]string, len(serFiles))
	for i, sf := range serFiles {
		series[i] = sf.Series
		serPaths[sf.Series.ID] = sf.Path
	}
	sched := channel.NewSchedule(series...)

	client := sponsorblock.New()
	var categories []sponsorblock.Category
	if *cats != "" {
		for _, c := range strings.Split(*cats, ",") {
			categories = append(categories, sponsorblock.Category(strings.TrimSpace(c)))
		}
	}
	segments, err := client.GetSegments(videoID, categories)
	if err != nil {
		slog.Error("failed to get segments", "video", videoID, "err", err)
		os.Exit(1)
	}
	if len(segments) == 0 {
		fmt.Printf("no segments found for %s\n", videoID)
		return
	}

	fmt.Printf("Segments for %s:\n\n", videoID)
	for i, seg := range segments {
		start := time.Duration(seg.Segment[0] * float64(time.Second)).Truncate(time.Second)
		end := time.Duration(seg.Segment[1] * float64(time.Second)).Truncate(time.Second)
		fmt.Printf("  [%d] %-15s %s → %s  (votes: %d, locked: %v)\n",
			i+1, seg.Category, sponsorblock.FormatDuration(start), sponsorblock.FormatDuration(end), seg.Votes, seg.Locked == 1)
	}

	var input string
	if *apply != "" {
		input = *apply
	} else {
		fmt.Printf("\nApply which segments? (comma-separated numbers, or 'all'): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
	}

	selected := make(map[int]bool)
	if input == "" {
		// no segments selected, will clear segments
	} else if input == "all" {
		for i := range segments {
			selected[i] = true
		}
	} else {
		for _, s := range strings.Split(input, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || n < 1 || n > len(segments) {
				fmt.Fprintf(os.Stderr, "invalid selection: %s\n", s)
				os.Exit(1)
			}
			selected[n-1] = true
		}
	}

	// Collect cuts (time ranges to skip), sorted by start time.
	type cut struct{ start, end time.Duration }
	var cuts []cut
	for idx, seg := range segments {
		if !selected[idx] {
			continue
		}
		start := time.Duration(seg.Segment[0] * float64(time.Second)).Truncate(time.Second)
		end := time.Duration(seg.Segment[1] * float64(time.Second)).Truncate(time.Second)
		cuts = append(cuts, cut{start, end})
		fmt.Printf("  cutting: %s → %s (%s)\n", sponsorblock.FormatDuration(start), sponsorblock.FormatDuration(end), seg.Category)
	}

	// Find the video's length from the schedule.
	video, ok := sched.Find(channel.NewYoutubeSource(videoID))
	if !ok {
		fmt.Fprintf(os.Stderr, "video %s not found in series dir\n", videoID)
		os.Exit(1)
	}
	videoLength := video.Length

	var newSegments []channel.Clip
	if len(cuts) > 0 {
		// Sort cuts by start time and merge overlapping.
		sort.Slice(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
		merged := []cut{cuts[0]}
		for _, c := range cuts[1:] {
			last := &merged[len(merged)-1]
			if c.start <= last.end {
				if c.end > last.end {
					last.end = c.end
				}
			} else {
				merged = append(merged, c)
			}
		}

		// Build playback segments from the gaps between cuts.
		pos := time.Duration(0)
		for _, c := range merged {
			if c.start-pos >= *minDur {
				newSegments = append(newSegments, makeSegment(pos, c.start))
			}
			pos = c.end
		}
		// Final segment from last cut to end of video.
		if videoLength-pos >= *minDur {
			newSegments = append(newSegments, makeSegment(pos, videoLength))
		}

		// Clean up redundant values and empty segments.
		newSegments = store.CleanEpisode(channel.NewEpisode(channel.Source{}, videoLength).WithClips(newSegments...)).Clips
	}

	// Print result.
	if len(newSegments) == 0 {
		fmt.Println("\n  result: full video (no segments)")
	} else {
		fmt.Println("\n  result:")
		for _, seg := range newSegments {
			start := "0:00"
			if seg.Start != 0 {
				start = sponsorblock.FormatDuration(seg.Start)
			}
			end := sponsorblock.FormatDuration(videoLength)
			if seg.End != 0 {
				end = sponsorblock.FormatDuration(seg.End)
			}
			fmt.Printf("    %s → %s\n", start, end)
		}
	}

	// Update the series file that contains this episode.
	for _, ser := range sched.Series {
		seasons := ser.Seasons
		changed := false
		for i, season := range seasons {
			for j, ep := range season.Episodes {
				if ep.Source.ID == videoID {
					seasons[i].Episodes[j] = seasons[i].Episodes[j].WithClips(newSegments...)
					changed = true
				}
			}
		}
		if changed {
			ser.Seasons = seasons
			if err := store.SaveSeries(serPaths[ser.ID], ser); err != nil {
				slog.Error("failed to save series", "name", ser.Name, "err", err)
				os.Exit(1)
			}
			fmt.Printf("\nsaved to %s\n", ser.Name)
			return
		}
	}
}

func makeSegment(start, end time.Duration) channel.Clip {
	return channel.NewClip(start, end)
}
