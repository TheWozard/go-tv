package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"go-tv/internal/channel"
	"go-tv/internal/client/sponsorblock"
	"go-tv/internal/ui/components"
)

type SponsorBlock struct {
	Channel *channel.Channel
}

func (sb *SponsorBlock) Route(r chi.Router) {
	r.Get("/{videoID}", sb.getHandler)
	r.Post("/{videoID}", sb.postHandler)
}

type sbCut struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

func (sb *SponsorBlock) getHandler(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	segs, err := sponsorblock.New().GetSegments(videoID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

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

func (sb *SponsorBlock) postHandler(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")

	var req struct {
		Cuts []sbCut `json:"cuts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	cuts := make([]channel.CutRange, len(req.Cuts))
	for i, c := range req.Cuts {
		cuts[i] = channel.CutRange{
			Start: time.Duration(c.StartSeconds * float64(time.Second)).Truncate(time.Second),
			End:   time.Duration(c.EndSeconds * float64(time.Second)).Truncate(time.Second),
		}
	}

	updated, err := sb.Channel.ApplyCuts(videoID, cuts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(components.VideoCard(*updated, false, 0))
}
