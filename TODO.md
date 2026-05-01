- [ ] Add segment reset to editor
- [ ] Add remove segment to editor
- [ ] Add append to editor
- [ ] Enable reorder between groups
- [ ] Rename video
- [ ] Open active video in edit view
- [x] Add native touch support
- [x] Fullscreen helper for touch
- [ ] Support pushing events on progress check in to update open clients

Bug
- [ ] Clicking checkbox in SB UI does nothing. Have to click name.
- [ ] editor.js silently swallows saveOrder() errors — user gets no feedback on failure
- [ ] Jellyfin manual jump via editor may have empty streamURL (only set on auto-advance)
- [x] State not persisted after edits — playback position lost on crash (only saves on clean shutdown)

Incomplete
- [ ] ApplyCuts only works for YouTube — Jellyfin videos have no cut/SponsorBlock support
- [ ] No input validation on cut start/end values — negative or out-of-range floats accepted silently