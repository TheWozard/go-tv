package channel

import (
	"context"
	"errors"
	"sort"
	"time"
)

var (
	ErrVideoNotInSchedule = errors.New("video not in schedule")
	ErrNoNextFragment     = errors.New("no next fragment")
)

// Channel coordinates schedule and state for a single channel.
type Channel struct {
	schedule *Schedule
	state    *State
}

// NewChannel wraps a schedule and state into a Channel.
func NewChannel(schedule *Schedule, state *State) *Channel {
	return &Channel{schedule: schedule, state: state}
}

// CurrentState returns the active source, playback position, and stop point.
func (c *Channel) CurrentState() (source Source, position, stopAt time.Duration, ok bool) {
	src, pos := c.state.Get()
	frag, ok := c.schedule.Current(src, pos)
	if frag.Source.Equal(src) {
		frag.Start = pos
	}
	return frag.Source, frag.Start, frag.End, ok
}

// Playlists returns all playlists in the schedule.
func (c *Channel) Playlists() []Playlist {
	return c.schedule.AllItems()
}

// Progress records playback position. Ignored if source is stale.
func (c *Channel) Progress(source Source, position time.Duration) {
	c.state.SetPosition(source, position)
}

// Jump unconditionally moves playback to source at position.
func (c *Channel) Jump(source Source, position time.Duration) error {
	if _, ok := c.schedule.Find(source); !ok {
		return ErrVideoNotInSchedule
	}
	c.state.Jump(source, position)
	return nil
}

// Next advances playback past source to the next fragment.
func (c *Channel) Next(source Source, position time.Duration) error {
	frag, ok := c.schedule.Next(source, position)
	if !ok {
		return ErrNoNextFragment
	}
	c.state.Advance(source, frag.Source, frag.Start)
	return nil
}

// RenamePlaylist renames the playlist identified by oldName.
func (c *Channel) RenamePlaylist(oldName, newName string) error {
	items := c.schedule.AllItems()
	for i, it := range items {
		if it.Name == oldName {
			items[i].Name = newName
			break
		}
	}
	c.schedule.Update(items)
	return c.schedule.Save()
}

// ReorderSet describes a playlist's desired video ordering.
type ReorderSet struct {
	Name     string
	VideoIDs []string
}

// Reorder updates playlist order and saves.
func (c *Channel) Reorder(sets []ReorderSet) error {
	existing := make(map[string]Video)
	for _, v := range c.schedule.All() {
		existing[v.Source.ID] = v
	}
	items := make([]Playlist, len(sets))
	for i, s := range sets {
		videos := make([]Video, 0, len(s.VideoIDs))
		for _, id := range s.VideoIDs {
			if v, ok := existing[id]; ok {
				videos = append(videos, v)
			}
		}
		items[i] = Playlist{Name: s.Name, Videos: videos}
	}
	c.schedule.Update(items)
	return c.schedule.Save()
}

// CutRange is a time range to remove from a video.
type CutRange struct {
	Start time.Duration
	End   time.Duration
}

// ApplyCuts converts cut ranges into keep-segments, updates the schedule, and saves.
// Returns the updated Video or an error if the video is not found or save fails.
func (c *Channel) ApplyCuts(videoID string, cuts []CutRange) (*Video, error) {
	video, ok := c.schedule.Find(NewYoutubeSource(videoID))
	if !ok {
		return nil, ErrVideoNotInSchedule
	}

	const minDur = 10 * time.Second
	var newSegments []Segment

	if len(cuts) > 0 {
		sort.Slice(cuts, func(i, j int) bool { return cuts[i].Start < cuts[j].Start })
		merged := []CutRange{cuts[0]}
		for _, cr := range cuts[1:] {
			last := &merged[len(merged)-1]
			if cr.Start <= last.End {
				if cr.End > last.End {
					last.End = cr.End
				}
			} else {
				merged = append(merged, cr)
			}
		}
		pos := time.Duration(0)
		for _, cut := range merged {
			if cut.Start-pos >= minDur {
				newSegments = append(newSegments, makeSeg(pos, cut.Start))
			}
			pos = cut.End
		}
		if video.Length.Duration-pos >= minDur {
			newSegments = append(newSegments, makeSeg(pos, video.Length.Duration))
		}
		v := &Video{Segments: newSegments, Length: video.Length}
		v.Clean()
		newSegments = v.Segments
	}

	items := c.schedule.AllItems()
	for i, item := range items {
		for j, v := range item.Videos {
			if v.Source.ID == videoID {
				items[i].Videos[j].Segments = newSegments
			}
		}
	}
	c.schedule.Update(items)
	if err := c.schedule.Save(); err != nil {
		return nil, err
	}

	updated, _ := c.schedule.Find(NewYoutubeSource(videoID))
	return updated, nil
}

// SaveState persists the current playback state to disk.
func (c *Channel) SaveState() error {
	return c.state.Save()
}

// StartAutoSave saves state on interval until ctx is cancelled.
func (c *Channel) StartAutoSave(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = c.state.Save()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func makeSeg(start, end time.Duration) Segment {
	seg := Segment{}
	if start > 0 {
		d := Duration{Duration: start}
		seg.Start = &d
	}
	if end > 0 {
		d := Duration{Duration: end}
		seg.End = &d
	}
	return seg
}

