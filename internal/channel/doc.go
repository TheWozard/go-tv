// Package channel models a linear TV channel built from video sources.
//
// A channel is defined by a [Schedule] — an ordered list of [Playlist] groups,
// each containing one or more [Video] entries. Videos may be subdivided into
// [Segment] ranges (e.g. after SponsorBlock processing) that describe which
// portions of the video should actually be played.
//
// At runtime the channel maintains a [State] (current source + playback
// position) that advances through [Fragment] values — the atomic unit of
// playback representing a single contiguous time range within a single video.
//
// Conceptual hierarchy:
//
//	Schedule
//	  └── Item (named group, e.g. a playlist)
//	        └── Video (single playable entry)
//	              └── Segment (playback window within the video)
//
// A [Fragment] is derived at runtime by combining a Video's Source with a
// Segment's time range. It is what the player actually receives.
//
// All exported methods on [Schedule] and [State] are safe for concurrent use.
package channel
