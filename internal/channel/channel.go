package channel

import (
	"errors"
	"sort"
	"time"
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

// CurrentState returns the active video ID, position, and stop point as scalars.
func (c *Channel) CurrentState() (videoID string, seconds, stopSeconds float64, ok bool) {
	source, position := c.state.Get()
	frag, ok := c.schedule.Current(source, position)
	if frag.Source.Equal(source) {
		frag.Start = position
	}
	return frag.Source.ID, frag.Start.Seconds(), frag.End.Seconds(), ok
}

// Playlists returns all playlists in the schedule.
func (c *Channel) Playlists() []Playlist {
	return c.schedule.AllItems()
}

// Progress records playback position. Ignored if videoID is stale.
func (c *Channel) Progress(videoID string, seconds float64) {
	c.state.SetPosition(NewYoutubeSource(videoID), secs(seconds))
}

// Jump unconditionally moves playback to videoID at seconds.
func (c *Channel) Jump(videoID string, seconds float64) error {
	if _, ok := c.schedule.Find(NewYoutubeSource(videoID)); !ok {
		return errors.New("video not in schedule")
	}
	c.state.Jump(NewYoutubeSource(videoID), secs(seconds))
	return nil
}

// Next advances playback past videoID to the next fragment.
func (c *Channel) Next(videoID string, seconds float64) error {
	src := NewYoutubeSource(videoID)
	frag, ok := c.schedule.Next(src, secs(seconds))
	if !ok {
		return errors.New("no next fragment")
	}
	c.state.Advance(src, frag.Source, frag.Start)
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
		return nil, errors.New("video not in schedule")
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

func secs(s float64) time.Duration {
	return time.Duration(s * float64(time.Second)).Truncate(time.Second)
}
