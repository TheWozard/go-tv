package channel

// SourceKind identifies the platform a video originates from.
type SourceKind string

const (
	SourceKindTest     SourceKind = "test"
	SourceKindYoutube  SourceKind = "youtube"
	SourceKindJellyfin SourceKind = "jellyfin"
)

// Source is a platform-scoped video identifier. The combination of Kind + ID uniquely
// addresses a single video across all supported backends.
type Source struct {
	Kind SourceKind
	ID   string
}

func NewTestSource(id string) Source {
	return Source{Kind: SourceKindTest, ID: id}
}

func NewYoutubeSource(id string) Source {
	return Source{Kind: SourceKindYoutube, ID: id}
}

func NewJellyfinSource(id string) Source {
	return Source{Kind: SourceKindJellyfin, ID: id}
}

func (s Source) Equal(o Source) bool {
	return s.Kind == o.Kind && s.ID == o.ID
}

// IsZero reports whether the source is the zero value (no ID set).
func (s Source) IsZero() bool {
	return s.ID == ""
}

// NewValidatedSource constructs a Source only if kind is a known value and id is non-empty.
func NewValidatedSource(kind SourceKind, id string) (Source, bool) {
	switch kind {
	case SourceKindTest, SourceKindYoutube, SourceKindJellyfin:
	default:
		return Source{}, false
	}
	if id == "" {
		return Source{}, false
	}
	return Source{Kind: kind, ID: id}, true
}
