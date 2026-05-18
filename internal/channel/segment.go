package channel

// Segment is the unit of playback: a specific Source played from Clip.Start to Clip.End.
// It is produced by the Schedule and consumed by the player.
type Segment struct {
	Source Source
	Clip   Clip
}
