# Adding Jellyfin as a Source

## 1. Config (`internal/config/config.go`)

Add a `Jellyfin` section with server URL and API key (or user credentials). These are needed to construct stream URLs and query metadata.

```yaml
jellyfin:
  url: "https://jellyfin.example.com"
  api_key: "abc123"
```

---

## 2. Channel / Source (`internal/channel/source.go`)

- Add `SourceKindJellyfin SourceKind = "jellyfin"`
- Add `NewJellyfinSource(id string) Source` helper
- Update `NewValidatedSource()` to accept it

The `id` field holds the Jellyfin item GUID. The schedule JSON format already handles this with no other changes.

---

## 3. Player Template (`internal/ui/components/player.templ`)

The Jellyfin stream URL can't be constructed purely in JS without exposing the API key to the client. Two options:

- **A (simpler):** Render a `data-stream-url` attribute server-side for Jellyfin sources, leaving YouTube sources to resolve via the IFrame API
- **B:** Add a `/api/stream-url` endpoint the JS calls

Option A is simplest — add a server-side helper that returns the stream URL for a given source.

---

## 4. State Template (`internal/ui/components/state.templ`)

Add `data-stream-url` when the source is Jellyfin, empty for YouTube (the IFrame API handles it). The JS reads this attribute to know which player backend to use.

---

## 5. JavaScript — Player Abstraction (`js/player/`)

This is the most significant change. Currently `state.player` is a YouTube IFrame player instance and `controls.js` calls `YT.PlayerState.PLAYING` directly.

Create `js/player/backends/youtube.js` and `js/player/backends/jellyfin.js`, each implementing a common interface:

```js
{ play(), pause(), seekTo(s), getCurrentTime(), getState() }
// getState() returns: 'playing' | 'paused' | 'ended' | 'other'
```

`player.js` becomes a factory: based on `sourceKind` in `applyState`, it initializes the right backend and assigns it to `state.player`.

`controls.js` and the tick loops stop touching `YT.PlayerState` directly and use `state.player.getState()` instead.

Jellyfin playback uses a `<video>` element with HLS.js (Jellyfin serves HLS streams). The stream URL comes from `data-stream-url`.

---

## 6. Editor / Video Card (`internal/ui/components/video_card.templ`)

- The YouTube watch link is hardcoded — make it conditional on source kind
- The SponsorBlock button should only render for `youtube` sources
- Add a Jellyfin web UI deep-link for Jellyfin sources

---

## 7. Import Tooling (`cmd/append/`)

The existing tool uses `yt-dlp`. A parallel `cmd/append-jellyfin/` (or a `--jellyfin` flag) would query the Jellyfin `/Items/{id}` API to fetch title and duration, then write the schedule entry with `kind: "jellyfin"`.

---

## Order of Attack

1. **Config + `SourceKind`** — unblocks everything else, low risk
2. **State template stream URL** — server-side only, no JS yet
3. **JS backend abstraction** — biggest chunk, isolate YouTube behind the interface first before adding Jellyfin
4. **Jellyfin JS backend** with HLS.js
5. **Video card template** fixes
6. **Import tool**
