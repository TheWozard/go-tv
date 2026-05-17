package channel

// SourceKind identifies the platform a video originates from.
type SourceKind string

const (
	SourceKindTest     SourceKind = "test"
	SourceKindYoutube  SourceKind = "youtube"
	SourceKindJellyfin SourceKind = "jellyfin"
)

func NewTestSource(id string) Source {
	return Source{kind: SourceKindTest, id: id}
}

func NewYoutubeSource(id string) Source {
	return Source{kind: SourceKindYoutube, id: id}
}

func NewJellyfinSource(id string) Source {
	return Source{kind: SourceKindJellyfin, id: id}
}

type Source struct {
	kind SourceKind
	id   string
}

func (s Source) GetKind() SourceKind {
	return s.kind
}

func (s Source) GetID() string {
	return s.id
}

func (s Source) Equal(o Source) bool {
	return s.kind == o.kind && s.id == o.id
}

func (s Source) IsZero() bool {
	return s.id == ""
}
