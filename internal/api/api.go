package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"go-tv/internal/channel"
	"go-tv/internal/client/sponsorblock"
	"go-tv/internal/ui"
	"go-tv/internal/ui/components"
)

// -------------------------------------------------------------------------
// API types
// -------------------------------------------------------------------------

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

// -------------------------------------------------------------------------
// Server
// -------------------------------------------------------------------------

func OpenChannel(r chi.Router, schedule *channel.Schedule, state *channel.State) {
	ch := &Channel{
		Schedule:    schedule,
		State:       state,
		Broadcaster: ui.NewBroadcaster(),
	}
	ch.Route(r)
}

type Channel struct {
	Schedule    *channel.Schedule
	State       *channel.State
	Broadcaster *ui.Broadcaster
}

func (s *Channel) Route(r chi.Router) {
	r.Get("/", s.playerHandler)
	r.Get("/edit", s.editHandler)
	r.Get("/sse/state", s.sseStateHandler)
	r.Post("/api/progress", s.progressHandler)
	r.Post("/api/next", s.nextHandler)
	r.Post("/api/jump", s.jumpHandler)
	r.Get("/api/sponsorblock/{videoID}", s.sponsorblockGetHandler)
	r.Post("/api/sponsorblock/{videoID}", s.sponsorblockPostHandler)
	r.Post("/api/schedule/rename", s.scheduleRenameHandler)
	r.Post("/api/schedule/reorder", s.scheduleReorderHandler)
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

type stateResult struct {
	VideoID     string
	Seconds     float64
	StopSeconds float64
}

func (s *Channel) currentState() (stateResult, bool) {
	source, position := s.State.Get()
	frag, ok := s.Schedule.Current(source, position)
	if frag.Source.Equal(source) {
		frag.Start = position
	}
	return stateResult{
		VideoID:     frag.Source.ID,
		Seconds:     frag.Start.Seconds(),
		StopSeconds: frag.End.Seconds(),
	}, ok
}

// broadcastState renders the current PlayerState fragment and pushes it to all SSE subscribers.
func (s *Channel) broadcastState(r *http.Request) {
	st, ok := s.currentState()
	if !ok {
		return
	}
	var buf bytes.Buffer
	if err := components.PlayerState(st.VideoID, st.Seconds, st.StopSeconds).Render(r.Context(), &buf); err != nil {
		return
	}
	s.Broadcaster.Broadcast(buf.String())
}

// -------------------------------------------------------------------------
// Page handlers
// -------------------------------------------------------------------------

func (s *Channel) playerHandler(w http.ResponseWriter, r *http.Request) {
	st, _ := s.currentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.Player(st.VideoID, st.Seconds, st.StopSeconds).Render(r.Context(), w)
}

func (s *Channel) editHandler(w http.ResponseWriter, r *http.Request) {
	sets := s.Schedule.AllItems()
	st, _ := s.currentState()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	components.Editor(sets, st.VideoID, st.Seconds).Render(r.Context(), w)
}

// -------------------------------------------------------------------------
// SSE handler
// -------------------------------------------------------------------------

func (s *Channel) sseStateHandler(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	// Push current state immediately so the player has something on connect
	if st, ok := s.currentState(); ok {
		if err := sse.PatchElementTempl(components.PlayerState(st.VideoID, st.Seconds, st.StopSeconds)); err != nil {
			return
		}
	}

	id, ch := s.Broadcaster.Subscribe()
	defer s.Broadcaster.Unsubscribe(id)

	for {
		select {
		case html, ok := <-ch:
			if !ok {
				return
			}
			if err := sse.PatchElements(html); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// -------------------------------------------------------------------------
// State handlers
// -------------------------------------------------------------------------

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
	s.broadcastState(r)
	w.WriteHeader(http.StatusNoContent)
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
	s.broadcastState(r)
	w.WriteHeader(http.StatusNoContent)
}

// -------------------------------------------------------------------------
// Schedule handlers
// -------------------------------------------------------------------------

func (s *Channel) scheduleRenameHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		OldName string `json:"old_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	items := s.Schedule.AllItems()
	for i, it := range items {
		if it.Name == req.OldName {
			items[i].Name = req.Name
			break
		}
	}
	s.Schedule.Update(items)
	if err := s.Schedule.Save(); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Channel) scheduleReorderHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			Name     string   `json:"name"`
			VideoIDs []string `json:"video_ids"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	existing := make(map[string]channel.Video)
	for _, v := range s.Schedule.All() {
		existing[v.Source.ID] = v
	}

	items := make([]channel.Playlist, len(req.Items))
	for i, ri := range req.Items {
		videos := make([]channel.Video, 0, len(ri.VideoIDs))
		for _, id := range ri.VideoIDs {
			if v, ok := existing[id]; ok {
				videos = append(videos, v)
			}
		}
		items[i] = channel.Playlist{Name: ri.Name, Videos: videos}
	}
	s.Schedule.Update(items)
	if err := s.Schedule.Save(); err != nil {
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -------------------------------------------------------------------------
// SponsorBlock handlers
// -------------------------------------------------------------------------

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

	// Return rendered HTML fragment for Datastar to merge into the panel slot
	sbSegs := make([]components.SBSegment, len(segs))
	for i, seg := range segs {
		sbSegs[i] = components.SBSegment{
			Index:        i,
			Category:     string(seg.Category),
			StartSeconds: seg.Segment[0],
			EndSeconds:   seg.Segment[1],
			Votes:        seg.Votes,
		}
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(components.SponsorBlockPanel(videoID, sbSegs))
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

	// Push updated video card fragment back to this client
	updatedPtr, ok := s.Schedule.Find(channel.NewYoutubeSource(videoID))
	if ok {
		updated := *updatedPtr
		st, _ := s.currentState()
		isActive := st.VideoID == videoID
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.VideoCard(updated, isActive, st.Seconds))
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
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
