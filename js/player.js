import { postProgress, postNext } from './player/api.js';
import { initControls } from './player/controls.js';

// state is shared with controls.js by reference so mutations here are visible there.
const state = {
  player: null,       // YouTube IFrame player instance, set in startHere()
  currentSource: null, // { kind, id } of the loaded video
  currentStop: 0,     // server-imposed stop time in seconds (0 = no stop)
};

// ytReady resolves when the YouTube IFrame API fires onYouTubeIframeAPIReady.
// startHere() awaits it before constructing the player.
const { promise: ytReady, resolve: ytApiLoaded } = Promise.withResolvers();

// ---- Playback control ----

// playingTime returns the current playback position in seconds, or null if the
// player is absent or not currently playing.
function playingTime() {
  if (!state.player || typeof state.player.getCurrentTime !== 'function') return null;
  if (state.player.getPlayerState() !== YT.PlayerState.PLAYING) return null;
  return state.player.getCurrentTime();
}

// advance moves to the next video, reporting the current position first.
function advance() {
  const seconds = (state.player && typeof state.player.getCurrentTime === 'function')
    ? state.player.getCurrentTime() : 0;
  postNext(state.currentSource, seconds);
}

// applyState syncs the player to server state. If the source hasn't changed
// it only seeks when positions are more than 5s apart (avoids seek loops).
function applyState(sourceKind, sourceId, seconds, stopSeconds) {
  state.currentStop = stopSeconds;
  if (state.currentSource?.kind === sourceKind && state.currentSource?.id === sourceId) {
    if (state.player && Math.abs(state.player.getCurrentTime() - seconds) > 5) {
      state.player.seekTo(seconds, true);
    }
    return;
  }
  state.currentSource = { kind: sourceKind, id: sourceId };
  if (sourceKind === 'youtube' && state.player) {
    state.player.loadVideoById({ videoId: sourceId, startSeconds: Math.floor(seconds) });
  }
}

// ---- State observer ----

// applyStateFromEl reads data-* attributes on #player-state and calls applyState.
function applyStateFromEl(el) {
  applyState(
    el.dataset.sourceKind,
    el.dataset.sourceId,
    parseFloat(el.dataset.position) || 0,
    parseFloat(el.dataset.stopAt) || 0,
  );
}

// Re-sync whenever HTMX swaps in a new #player-state fragment.
document.addEventListener('htmx:afterSwap', e => {
  if (e.detail.target?.id !== 'player-state') return;
  const el = document.getElementById('player-state');
  if (el) applyStateFromEl(el);
});

document.addEventListener('DOMContentLoaded', () => {
  const el = document.getElementById('player-state');
  if (el) applyStateFromEl(el);

  initControls(state, advance);

  const wrapper = document.getElementById('player-wrapper');

  // Advance check: frequent so stop-time is caught promptly.
  const advanceMs = parseInt(wrapper?.dataset.advanceRateMs, 10) || 1000;
  setInterval(() => {
    const t = playingTime();
    if (t === null) return;
    if (state.currentStop > 0 && t >= state.currentStop) advance();
  }, advanceMs);

  // Progress report: less frequent, just keeps the server in sync.
  const progressMs = parseInt(wrapper?.dataset.progressRateMs, 10) || 10000;
  setInterval(() => {
    const t = playingTime();
    if (t === null) return;
    postProgress(state.currentSource, t);
  }, progressMs);
});

// ---- YouTube IFrame API bootstrap ----

// Called by the YouTube IFrame API script once it has loaded.
window.onYouTubeIframeAPIReady = ytApiLoaded;

// startHere is called by the "Watch Here" button. It hides the start screen,
// waits for the YT API, then constructs the player at the server-supplied position.
window.startHere = async function () {
  document.getElementById('start-screen').style.display = 'none';
  document.getElementById('player-wrapper').style.display = 'block';

  await ytReady;

  const el = document.getElementById('player-state');
  state.currentSource = { kind: el?.dataset.sourceKind || '', id: el?.dataset.sourceId || '' };
  state.currentStop = parseFloat(el?.dataset.stopAt) || 0;

  state.player = new YT.Player('player', {
    videoId: state.currentSource.id,
    playerVars: { start: Math.floor(parseFloat(el?.dataset.position) || 0), autoplay: 1, controls: 0 },
    events: {
      onStateChange(event) {
        if (event.data === YT.PlayerState.ENDED) advance();
      },
      onError() { advance(); },
    },
  });
};
