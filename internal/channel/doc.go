// Package channel models a linear TV channel built from video sources.
//
// A channel is defined by a [Schedule] — a collection of [Series], each loaded
// from its own JSON file. Each Series contains one or more [Season] groups,
// and each Season contains one or more [Episode] entries. Episodes may be
// subdivided into [Segment] ranges (e.g. after SponsorBlock processing) that
// describe which portions should actually be played.
//
// At runtime the channel maintains a [State] (current source + playback
// position) that advances through [Fragment] values — the atomic unit of
// playback representing a single contiguous time range within a single episode.
//
// Conceptual hierarchy:
//
//	Schedule
//	  └── Series (separate JSON file per TV show)
//	        └── Season (named group, e.g. "Season 1")
//	              └── Episode (single playable entry)
//	                    └── Segment (playback window within the episode)
//
// A [Fragment] is derived at runtime by combining an Episode's Source with a
// Segment's time range. It is what the player actually receives.
//
// When an episode ends, the next fragment is chosen based on the episode's
// Continue flag: if set, the next episode in the same season plays; otherwise
// a series is chosen at random and its first episode plays.
//
// All exported methods on [Schedule], [Series], and [State] are safe for concurrent use.
package channel
