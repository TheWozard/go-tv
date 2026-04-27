// sponsorblock fetches segments from SponsorBlock for a video and updates
// the schedule. Each selected segment splits the video's playback, creating
// gaps where skipped content lives. Segments shorter than -min are discarded.
//
// Usage:
//
//	go run ./cmd/sponsorblock [flags] <video-id>
//
// Flags:
//
//	-schedule  path to schedule file (default: schedule.json)
//	-min       minimum segment duration to keep (default: 10s)
//	-apply     pre-select segments (comma-separated numbers, or 'all')
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-tv/internal/channel"
	"go-tv/internal/client/sponsorblock"
)

func main() {
	schedPath := flag.String("s", "schedule.json", "path to schedule file")
	minDur := flag.Duration("m", 10*time.Second, "minimum segment duration to keep")
	apply := flag.String("a", "", "segments to apply (comma-separated numbers, or 'all')")
	cats := flag.String("c", "", "categories to fetch (comma-separated; default: all)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: sponsorblock [flags] <video-id>")
		os.Exit(1)
	}
	videoID := flag.Arg(0)

	sched, err := channel.LoadSchedule(*schedPath)
	if err != nil {
		log.Fatal(err)
	}

	client := sponsorblock.New()
	var categories []sponsorblock.Category
	if *cats != "" {
		for _, c := range strings.Split(*cats, ",") {
			categories = append(categories, sponsorblock.Category(strings.TrimSpace(c)))
		}
	}
	segments, err := client.GetSegments(videoID, categories)
	if err != nil {
		log.Fatalf("%s: %v", videoID, err)
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
		fmt.Fprintf(os.Stderr, "video %s not found in schedule\n", videoID)
		os.Exit(1)
	}
	videoLength := video.Length.Duration

	var newSegments []channel.Segment
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
		v := &channel.Video{Segments: newSegments, Length: video.Length}
		v.Clean()
		newSegments = v.Segments
	}

	// Print result.
	if len(newSegments) == 0 {
		fmt.Println("\n  result: full video (no segments)")
	} else {
		fmt.Println("\n  result:")
		for _, seg := range newSegments {
			start := "0:00"
			if seg.Start != nil {
				start = sponsorblock.FormatDuration(seg.Start.Duration)
			}
			end := sponsorblock.FormatDuration(videoLength)
			if seg.End != nil {
				end = sponsorblock.FormatDuration(seg.End.Duration)
			}
			fmt.Printf("    %s → %s\n", start, end)
		}
	}

	// Update schedule.
	items := sched.AllItems()
	for i, item := range items {
		for j, v := range item.Videos {
			if v.Source.ID == videoID {
				items[i].Videos[j].Segments = newSegments
			}
		}
	}

	sched.Update(items)
	if err := sched.Save(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nsaved to %s\n", *schedPath)
}

func makeSegment(start, end time.Duration) channel.Segment {
	seg := channel.Segment{}
	if start > 0 {
		d := channel.Duration{Duration: start}
		seg.Start = &d
	}
	if end > 0 {
		d := channel.Duration{Duration: end}
		seg.End = &d
	}
	return seg
}
