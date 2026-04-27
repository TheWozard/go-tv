// Package sponsorblock provides a client for the SponsorBlock API.
// See https://wiki.sponsor.ajay.app/w/API_Docs
package sponsorblock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://sponsor.ajay.app"

// Category represents a segment category.
type Category string

const (
	CategorySponsor       Category = "sponsor"
	CategorySelfpromo     Category = "selfpromo"
	CategoryInteraction   Category = "interaction"
	CategoryIntro         Category = "intro"
	CategoryOutro         Category = "outro"
	CategoryPreview       Category = "preview"
	CategoryMusicOfftopic Category = "music_offtopic"
	CategoryFiller        Category = "filler"
)

// AllCategories returns all available segment categories.
func AllCategories() []Category {
	return []Category{
		CategorySponsor,
		CategorySelfpromo,
		CategoryInteraction,
		CategoryIntro,
		CategoryOutro,
		CategoryPreview,
		CategoryMusicOfftopic,
		CategoryFiller,
	}
}

// ActionType represents what action to take for a segment.
type ActionType string

const (
	ActionSkip  ActionType = "skip"
	ActionMute  ActionType = "mute"
	ActionFull  ActionType = "full"
	ActionPoi   ActionType = "poi"
	ActionChapter ActionType = "chapter"
)

// Segment represents a single skip segment returned by the API.
type Segment struct {
	Segment    [2]float64 `json:"segment"`
	UUID       string     `json:"UUID"`
	Category   Category   `json:"category"`
	ActionType ActionType `json:"actionType"`
	Locked     int        `json:"locked"`
	Votes      int        `json:"votes"`
	Description string   `json:"description"`
}

// Start returns the segment start time.
func (s Segment) Start() time.Duration {
	return time.Duration(s.Segment[0] * float64(time.Second))
}

// End returns the segment end time.
func (s Segment) End() time.Duration {
	return time.Duration(s.Segment[1] * float64(time.Second))
}

// Client is a SponsorBlock API client.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
}

// New creates a new SponsorBlock client.
func New() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		BaseURL:    baseURL,
	}
}

// GetSegments retrieves skip segments for a video.
// If categories is nil, all categories are requested.
func (c *Client) GetSegments(videoID string, categories []Category) ([]Segment, error) {
	if categories == nil {
		categories = AllCategories()
	}

	catJSON, err := json.Marshal(categories)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/api/skipSegments?videoID=%s&categories=%s",
		c.BaseURL,
		url.QueryEscape(videoID),
		url.QueryEscape(string(catJSON)),
	)

	resp, err := c.HTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("sponsorblock: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no segments for this video
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: unexpected status %d", resp.StatusCode)
	}

	var segments []Segment
	if err := json.NewDecoder(resp.Body).Decode(&segments); err != nil {
		return nil, fmt.Errorf("sponsorblock: decode response: %w", err)
	}
	return segments, nil
}

// SearchSegments retrieves segments with advanced filtering options.
type SearchParams struct {
	VideoID  string
	MinVotes *int
	MaxVotes *int
	Locked   *bool
	Hidden   *bool
	Page     int
}

// SearchResult represents a segment from the search endpoint.
type SearchResult struct {
	UUID          string     `json:"UUID"`
	Segment       [2]float64 `json:"segment"`
	Category      Category   `json:"category"`
	ActionType    ActionType `json:"actionType"`
	Locked        int        `json:"locked"`
	Votes         int        `json:"votes"`
	Hidden        int        `json:"hidden"`
	UserID        string     `json:"userID"`
	TimeSubmitted int64      `json:"timeSubmitted"`
	Description   string     `json:"description"`
}

// SearchResponse is the response from the search endpoint.
type SearchResponse struct {
	Segments   []SearchResult `json:"segments"`
	Page       int            `json:"page"`
	SegmentCount int          `json:"segmentCount"`
}

// SearchSegments performs an advanced segment search for a video.
func (c *Client) SearchSegments(params SearchParams) (*SearchResponse, error) {
	vals := url.Values{}
	vals.Set("videoID", params.VideoID)
	if params.MinVotes != nil {
		vals.Set("minVotes", fmt.Sprintf("%d", *params.MinVotes))
	}
	if params.MaxVotes != nil {
		vals.Set("maxVotes", fmt.Sprintf("%d", *params.MaxVotes))
	}
	if params.Locked != nil {
		if *params.Locked {
			vals.Set("locked", "1")
		} else {
			vals.Set("locked", "0")
		}
	}
	if params.Hidden != nil {
		if *params.Hidden {
			vals.Set("hidden", "1")
		} else {
			vals.Set("hidden", "0")
		}
	}
	if params.Page > 0 {
		vals.Set("page", fmt.Sprintf("%d", params.Page))
	}

	u := fmt.Sprintf("%s/api/searchSegments?%s", c.BaseURL, vals.Encode())

	resp, err := c.HTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("sponsorblock: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &SearchResponse{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: unexpected status %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sponsorblock: decode response: %w", err)
	}
	return &result, nil
}

// FormatDuration formats a duration as HH:MM:SS or MM:SS.
func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// FormatCategories returns a human-readable string of categories.
func FormatCategories(cats []Category) string {
	strs := make([]string, len(cats))
	for i, c := range cats {
		strs[i] = string(c)
	}
	return strings.Join(strs, ", ")
}
