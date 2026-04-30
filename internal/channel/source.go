package channel

// SourceKind identifies the platform a video originates from.
type SourceKind string

const (
	SourceKindTest     SourceKind = "test"
	SourceKindYoutube  SourceKind = "youtube"
	SourceKindJellyfin SourceKind = "jellyfin"
)

// newSource creates a source with the given kind and ID.
func newSource(kind SourceKind, id string) Source {
	return Source{Kind: kind, ID: id}
}

// NewValidatedSource creates a source after checking that kind is recognized
// and id is non-empty. Returns false if either check fails.
func NewValidatedSource(kind SourceKind, id string) (Source, bool) {
	if id == "" {
		return Source{}, false
	}
	switch kind {
	case SourceKindYoutube, SourceKindTest, SourceKindJellyfin:
		return newSource(kind, id), true
	default:
		return Source{}, false
	}
}

func NewTestSource(id string) Source {
	return newSource(SourceKindTest, id)
}

func NewYoutubeSource(id string) Source {
	return newSource(SourceKindYoutube, id)
}

func NewJellyfinSource(id string) Source {
	return newSource(SourceKindJellyfin, id)
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
