package channel

import (
	"bytes"
	"encoding/json"
)

// SourceKind identifies the platform a video originates from.
type SourceKind string

const (
	SourceKindYoutube SourceKind = "youtube"
)

// NewSource creates a YouTube source with the given video ID.
func NewSource(id string) Source {
	return Source{
		Kind: SourceKindYoutube,
		ID:   id,
	}
}

// Source identifies a video on a specific platform.
// It serializes to JSON in two formats: a bare string "id" (legacy, assumes
// YouTube) or an object {"kind":"youtube","id":"..."}.
type Source struct {
	Kind SourceKind `json:"kind"`
	ID   string     `json:"id"`
}

// Equal reports whether two sources refer to the same video.
func (s Source) Equal(o Source) bool {
	return s.Kind == o.Kind && s.ID == o.ID
}

// Clean fills in missing fields with defaults (YouTube kind).
func (s *Source) Clean() {
	if s.Kind == "" {
		s.Kind = SourceKindYoutube
	}
}

// UnmarshalJSON supports both the legacy bare-string format ("videoID") and
// the full object format ({"kind":"youtube","id":"videoID"}).
func (s *Source) UnmarshalJSON(b []byte) error {
	if bytes.IndexRune(b, '"') == 0 {
		// Legacy format
		var id string
		if err := json.Unmarshal(b, &id); err != nil {
			return err
		}
		s.ID = id
		s.Clean()
	} else {
		type raw Source // prevents infinite recursion
		var ingest raw
		if err := json.Unmarshal(b, &ingest); err != nil {
			return err
		}
		*s = Source(ingest)
	}
	return nil
}
