// append-jellyfin searches a Jellyfin server and appends selected items to a schedule.
//
// Usage:
//
//	go run ./cmd/append-jellyfin [flags] <search query>
//
// Flags:
//
//	-config    path to config file (default: config.yaml)
//	-schedule  path to schedule file (default: schedule.json)
//	-limit     max search results to display (default: 20)
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
	"strconv"
	"strings"
	"time"

	"go-tv/internal/channel"
	"go-tv/internal/config"
)

type jfItem struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	Type         string `json:"Type"`
	RunTimeTicks *int64 `json:"RunTimeTicks"` // 100ns ticks; nil when unknown
	SeriesName   string `json:"SeriesName"`   // non-empty for Episodes
	ParentIndexNumber *int `json:"ParentIndexNumber"` // season number
	IndexNumber       *int `json:"IndexNumber"`       // episode number
}

func (item jfItem) displayTitle() string {
	if item.SeriesName != "" {
		s, e := 0, 0
		if item.ParentIndexNumber != nil {
			s = *item.ParentIndexNumber
		}
		if item.IndexNumber != nil {
			e = *item.IndexNumber
		}
		return fmt.Sprintf("%s S%02dE%02d – %s", item.SeriesName, s, e, item.Name)
	}
	return item.Name
}

func (item jfItem) duration() channel.Duration {
	if item.RunTimeTicks == nil || *item.RunTimeTicks == 0 {
		return channel.Duration{}
	}
	return channel.Duration{Duration: time.Duration(*item.RunTimeTicks * 100).Truncate(time.Second)}
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	schedPath := flag.String("schedule", "schedule.json", "path to schedule file")
	limit := flag.Int("limit", 20, "max results to show")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: append-jellyfin [flags] <search query>")
		os.Exit(1)
	}
	query := strings.Join(flag.Args(), " ")

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}
	if cfg.Jellyfin.URL == "" {
		slog.Error("jellyfin.url not set in config")
		os.Exit(1)
	}

	items := search(cfg.Jellyfin, query, *limit)
	if len(items) == 0 {
		fmt.Println("no results found")
		return
	}

	for i, item := range items {
		dur := ""
		if d := item.duration(); d.Duration > 0 {
			dur = fmt.Sprintf("  [%s]", d.Duration.Round(time.Second))
		}
		fmt.Printf("  %2d. %-60s %s%s\n", i+1, item.displayTitle(), item.Type, dur)
	}

	selected := prompt(len(items))
	if len(selected) == 0 {
		fmt.Println("nothing selected")
		return
	}

	sched, err := channel.LoadSchedule(*schedPath)
	if os.IsNotExist(err) {
		sched = channel.NewSchedule(*schedPath)
	} else if err != nil {
		slog.Error("failed to load schedule", "err", err)
		os.Exit(1)
	}

	videos := make([]channel.Video, 0, len(selected))
	for _, idx := range selected {
		item := items[idx]
		v := channel.Video{
			Source: channel.NewJellyfinSource(item.ID),
			Title:  item.displayTitle(),
			Length: item.duration(),
		}
		videos = append(videos, v)
		fmt.Printf("  + %s  %s\n", item.ID, item.displayTitle())
	}

	name := items[selected[0]].displayTitle()
	if len(selected) > 1 {
		name = query
	}

	sched.Update(append(sched.AllItems(), channel.Playlist{Name: name, Videos: videos}))
	fmt.Printf("added %q (%d video(s))\n", name, len(videos))

	if err := sched.Save(); err != nil {
		slog.Error("failed to save schedule", "err", err)
		os.Exit(1)
	}
	fmt.Printf("appended to %s\n", *schedPath)
}

func search(jf config.Jellyfin, query string, limit int) []jfItem {
	u, err := url.Parse(jf.URL + "/Items")
	if err != nil {
		slog.Error("invalid jellyfin url", "err", err)
		os.Exit(1)
	}
	q := u.Query()
	q.Set("searchTerm", query)
	q.Set("IncludeItemTypes", "Movie,Episode,Series")
	q.Set("Recursive", "true")
	q.Set("Limit", strconv.Itoa(limit))
	q.Set("Fields", "RunTimeTicks,ParentIndexNumber,IndexNumber,SeriesName")
	q.Set("api_key", jf.APIKey)
	u.RawQuery = q.Encode()

	resp, err := jf.HTTPClient().Get(u.String())
	if err != nil {
		slog.Error("jellyfin search failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		slog.Error("jellyfin search failed", "status", resp.Status)
		os.Exit(1)
	}

	var result struct {
		Items []jfItem `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("parsing response", "err", err)
		os.Exit(1)
	}
	return result.Items
}

// prompt asks the user to pick results and returns 0-based indices.
// Accepts: "all", a single number, or a comma-separated list like "1,3,5".
func prompt(total int) []int {
	fmt.Printf("\nselect [1-%d], comma-separated, or 'all': ", total)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return nil
	}
	if strings.EqualFold(input, "all") {
		out := make([]int, total)
		for i := range out {
			out[i] = i
		}
		return out
	}
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(input, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 1 || n > total {
			fmt.Fprintf(os.Stderr, "skipping invalid selection %q\n", part)
			continue
		}
		if !seen[n-1] {
			out = append(out, n-1)
			seen[n-1] = true
		}
	}
	return out
}
