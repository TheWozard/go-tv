package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"go-tv/internal/channel"
	"go-tv/internal/client/sponsorblock"
)

// -------------------------------------------------------------------------
// API types
// -------------------------------------------------------------------------

type stateResponse struct {
	VideoID     string  `json:"video_id"`
	Seconds     float64 `json:"seconds"`
	StopSeconds float64 `json:"stop_seconds"`
}

type nextRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

type progressRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

type jumpRequest struct {
	VideoID string  `json:"video_id"`
	Seconds float64 `json:"seconds"`
}

// wireSegment is the wire format for a playback segment.
type wireSegment struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds,omitempty"`
}

// scheduleVideo is the wire format for a single video within an item.
type scheduleVideo struct {
	ID            string        `json:"id,omitempty"`
	Title         string        `json:"title,omitempty"`
	Segments      []wireSegment `json:"segments,omitempty"`
	LengthSeconds float64       `json:"length_seconds,omitempty"`
}

// scheduleItem is the wire format for a schedule item (named group of videos).
type scheduleItem struct {
	Name   string          `json:"name"`
	Videos []scheduleVideo `json:"videos"`
}

// -------------------------------------------------------------------------
// Server
// -------------------------------------------------------------------------

func OpenChannel(r chi.Router, schedule *channel.Schedule, state *channel.State) {
	channel := &Channel{
		Schedule: schedule,
		State:    state,
	}
	channel.Route(r)
}

type Channel struct {
	Schedule *channel.Schedule
	State    *channel.State
}

func (s *Channel) Route(r chi.Router) {
	r.Get("/edit", s.editHandler)
	r.Get("/api/state", s.stateHandler)
	r.Get("/api/schedule", s.scheduleGetHandler)
	r.Post("/api/schedule", s.schedulePostHandler)
	r.Post("/api/progress", s.progressHandler)
	r.Post("/api/next", s.nextHandler)
	r.Post("/api/jump", s.jumpHandler)
	r.Get("/api/sponsorblock/{videoID}", s.sponsorblockGetHandler)
	r.Post("/api/sponsorblock/{videoID}", s.sponsorblockPostHandler)
	r.Handle("/*", http.FileServer(http.Dir("static")))
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *Channel) currentState() (stateResponse, bool) {
	source, position := s.State.Get()
	frag, ok := s.Schedule.Current(source, position)
	if frag.Source.Equal(source) {
		frag.Start = position
	}
	return stateResponse{
		VideoID:     frag.Source.ID,
		Seconds:     frag.Start.Seconds(),
		StopSeconds: frag.End.Seconds(),
	}, ok
}

func videoToWire(v channel.Video) scheduleVideo {
	w := scheduleVideo{
		ID:            v.Source.ID,
		Title:         v.Title,
		LengthSeconds: v.Length.Seconds(),
	}
	for _, seg := range v.Segments {
		ws := wireSegment{}
		if seg.Start != nil {
			ws.StartSeconds = seg.Start.Seconds()
		}
		if seg.End != nil {
			ws.EndSeconds = seg.End.Seconds()
		}
		w.Segments = append(w.Segments, ws)
	}
	return w
}

func wireToVideo(w scheduleVideo) channel.Video {
	v := channel.Video{
		Source: channel.NewYoutubeSource(w.ID),
		Title:  w.Title,
		Length: channel.Duration{Duration: time.Duration(w.LengthSeconds * float64(time.Second)).Truncate(time.Second)},
	}
	for _, ws := range w.Segments {
		seg := channel.Segment{}
		if ws.StartSeconds > 0 {
			d := channel.Duration{Duration: time.Duration(ws.StartSeconds * float64(time.Second)).Truncate(time.Second)}
			seg.Start = &d
		}
		if ws.EndSeconds > 0 {
			d := channel.Duration{Duration: time.Duration(ws.EndSeconds * float64(time.Second)).Truncate(time.Second)}
			seg.End = &d
		}
		v.Segments = append(v.Segments, seg)
	}
	return v
}

func itemToWire(it channel.Playlist) scheduleItem {
	videos := make([]scheduleVideo, len(it.Videos))
	for i, v := range it.Videos {
		videos[i] = videoToWire(v)
	}
	return scheduleItem{Name: it.Name, Videos: videos}
}

func wireToItem(w scheduleItem) channel.Playlist {
	videos := make([]channel.Video, len(w.Videos))
	for i, v := range w.Videos {
		videos[i] = wireToVideo(v)
	}
	return channel.Playlist{Name: w.Name, Videos: videos}
}

// -------------------------------------------------------------------------
// Handlers
// -------------------------------------------------------------------------

func (s *Channel) editHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/edit.html")
}

func (s *Channel) stateHandler(w http.ResponseWriter, r *http.Request) {
	resp, ok := s.currentState()
	if !ok {
		http.Error(w, "current video not in schedule", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}

func (s *Channel) scheduleGetHandler(w http.ResponseWriter, r *http.Request) {
	its := s.Schedule.AllItems()
	entries := make([]scheduleItem, len(its))
	for i, it := range its {
		entries[i] = itemToWire(it)
	}
	writeJSON(w, map[string]any{"items": entries}, http.StatusOK)
}

func (s *Channel) schedulePostHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []scheduleItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Build a lookup of existing videos so we can preserve segments.
	existing := make(map[string]channel.Video)
	for _, v := range s.Schedule.All() {
		existing[v.Source.ID] = v
	}

	items := make([]channel.Playlist, len(body.Items))
	for i, entry := range body.Items {
		videos := make([]channel.Video, len(entry.Videos))
		for j, wv := range entry.Videos {
			if ev, ok := existing[wv.ID]; ok {
				// Preserve existing video data (segments, length); allow title/name changes.
				ev.Title = wv.Title
				videos[j] = ev
			} else {
				videos[j] = wireToVideo(wv)
			}
		}
		items[i] = channel.Playlist{Name: entry.Name, Videos: videos}
	}
	s.Schedule.Update(items)
	if err := s.Schedule.Save(); err != nil {
		http.Error(w, "failed to save schedule", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Channel) progressHandler(w http.ResponseWriter, r *http.Request) {
	var req progressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.State.SetPosition(channel.NewYoutubeSource(req.VideoID), time.Duration(req.Seconds*float64(time.Second)))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Channel) jumpHandler(w http.ResponseWriter, r *http.Request) {
	var req jumpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if _, ok := s.Schedule.Find(channel.NewYoutubeSource(req.VideoID)); !ok {
		http.Error(w, "video not in schedule", http.StatusBadRequest)
		return
	}
	s.State.Jump(channel.NewYoutubeSource(req.VideoID), time.Duration(req.Seconds*float64(time.Second)))
	resp, ok := s.currentState()
	if !ok {
		http.Error(w, "current video not in schedule", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}

func (s *Channel) nextHandler(w http.ResponseWriter, r *http.Request) {
	var req nextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VideoID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	pos := time.Duration(req.Seconds * float64(time.Second))
	frag, ok := s.Schedule.Next(channel.NewYoutubeSource(req.VideoID), pos)
	if !ok {
		http.Error(w, "no next fragment found", http.StatusInternalServerError)
		return
	}
	s.State.Advance(channel.NewYoutubeSource(req.VideoID), frag.Source, frag.Start)
	resp, ok := s.currentState()
	if !ok {
		http.Error(w, "current video not in schedule", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}

// -------------------------------------------------------------------------
// SponsorBlock handlers
// -------------------------------------------------------------------------

// sbSegmentWire is a single SponsorBlock segment returned to the client.
type sbSegmentWire struct {
	Index        int     `json:"index"`
	Category     string  `json:"category"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Votes        int     `json:"votes"`
	Locked       bool    `json:"locked"`
}

// sbCut is a time range the client wants to skip.
type sbCut struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

func (s *Channel) sponsorblockGetHandler(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	segs, err := sponsorblock.New().GetSegments(videoID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	result := make([]sbSegmentWire, len(segs))
	for i, seg := range segs {
		result[i] = sbSegmentWire{
			Index:        i,
			Category:     string(seg.Category),
			StartSeconds: seg.Segment[0],
			EndSeconds:   seg.Segment[1],
			Votes:        seg.Votes,
			Locked:       seg.Locked == 1,
		}
	}
	writeJSON(w, map[string]any{"segments": result}, http.StatusOK)
}

func (s *Channel) sponsorblockPostHandler(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")

	var req struct {
		Cuts []sbCut `json:"cuts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	video, ok := s.Schedule.Find(channel.NewYoutubeSource(videoID))
	if !ok {
		http.Error(w, "video not in schedule", http.StatusNotFound)
		return
	}

	const minDur = 10 * time.Second
	videoLength := video.Length.Duration

	type cut struct{ start, end time.Duration }
	cuts := make([]cut, 0, len(req.Cuts))
	for _, c := range req.Cuts {
		cuts = append(cuts, cut{
			start: time.Duration(c.StartSeconds * float64(time.Second)).Truncate(time.Second),
			end:   time.Duration(c.EndSeconds * float64(time.Second)).Truncate(time.Second),
		})
	}

	var newSegments []channel.Segment
	if len(cuts) > 0 {
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

		pos := time.Duration(0)
		for _, c := range merged {
			if c.start-pos >= minDur {
				newSegments = append(newSegments, sbMakeSeg(pos, c.start))
			}
			pos = c.end
		}
		if videoLength-pos >= minDur {
			newSegments = append(newSegments, sbMakeSeg(pos, videoLength))
		}

		v := &channel.Video{Segments: newSegments, Length: video.Length}
		v.Clean()
		newSegments = v.Segments
	}

	items := s.Schedule.AllItems()
	for i, item := range items {
		for j, v := range item.Videos {
			if v.Source.ID == videoID {
				items[i].Videos[j].Segments = newSegments
			}
		}
	}
	s.Schedule.Update(items)
	if err := s.Schedule.Save(); err != nil {
		http.Error(w, "failed to save schedule", http.StatusInternalServerError)
		return
	}

	wireSegs := make([]wireSegment, len(newSegments))
	for i, seg := range newSegments {
		ws := wireSegment{}
		if seg.Start != nil {
			ws.StartSeconds = seg.Start.Seconds()
		}
		if seg.End != nil {
			ws.EndSeconds = seg.End.Seconds()
		}
		wireSegs[i] = ws
	}
	writeJSON(w, map[string]any{"segments": wireSegs}, http.StatusOK)
}

func sbMakeSeg(start, end time.Duration) channel.Segment {
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
