import { postProgress, postNext } from './player/api.js';
import { initControls } from './player/controls.js';
import { createYoutubeBackend } from './player/backends/youtube.js';
import { createJellyfinBackend } from './player/backends/jellyfin.js';

// state is shared with controls.js by reference so mutations here are visible there.
const state = {
  player: null,        // backend instance ({ play, pause, seekTo, getCurrentTime, getState })
  currentSource: null, // { kind, id } of the loaded video
  currentStop: 0,      // server-imposed stop time in seconds (0 = no stop)
};

// ytReady resolves when the YouTube IFrame API fires onYouTubeIframeAPIReady.
const { promise: ytReady, resolve: ytApiLoaded } = Promise.withResolvers();

// ---- Playback control ----

// playingTime returns the current playback position in seconds, or null if the
// player is absent or not currently playing.
function playingTime() {
  if (!state.player) return null;
  if (state.player.getState() !== 'playing') return null;
  return state.player.getCurrentTime();
}

// advance moves to the next video, reporting the current position first.
function advance() {
  const seconds = state.player ? state.player.getCurrentTime() : 0;
  postNext(state.currentSource, seconds);
}

// applyState syncs the player to server state. If the source hasn't changed
// it only seeks when positions are more than 5s apart (avoids seek loops).
function applyState(sourceKind, sourceId, seconds, stopSeconds, streamURL) {
  state.currentStop = stopSeconds;
  if (state.currentSource?.kind === sourceKind && state.currentSource?.id === sourceId) {
    if (state.player && Math.abs(state.player.getCurrentTime() - seconds) > 5) {
      state.player.seekTo(seconds);
    }
    return;
  }
  state.currentSource = { kind: sourceKind, id: sourceId };
  if (state.player) {
    if (sourceKind === 'youtube') {
      state.player.loadVideo(sourceId, seconds);
    }
    // For jellyfin a source change means a different stream URL — re-init.
    if (sourceKind === 'jellyfin') {
      createJellyfinBackend('player', streamURL, seconds, { onEnded: advance, onError: advance })
        .then(backend => { state.player = backend; })
        .catch(() => advance());
    }
  }
}

// ---- State observer ----

function applyStateFromEl(el) {
  applyState(
    el.dataset.sourceKind,
    el.dataset.sourceId,
    parseFloat(el.dataset.position) || 0,
    parseFloat(el.dataset.stopAt) || 0,
    el.dataset.streamUrl || '',
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

window.onYouTubeIframeAPIReady = ytApiLoaded;

// startHere is called by the "Watch Here" button.
window.startHere = async function () {
  document.getElementById('start-screen').style.display = 'none';
  document.getElementById('player-wrapper').style.display = 'block';

  const el = document.getElementById('player-state');
  const sourceKind = el?.dataset.sourceKind || '';
  const sourceId   = el?.dataset.sourceId || '';
  const streamURL  = el?.dataset.streamUrl || '';
  const position   = parseFloat(el?.dataset.position) || 0;
  const stopAt     = parseFloat(el?.dataset.stopAt) || 0;

  state.currentSource = { kind: sourceKind, id: sourceId };
  state.currentStop = stopAt;

  if (sourceKind === 'jellyfin') {
    state.player = await createJellyfinBackend('player', streamURL, position, {
      onEnded: advance,
      onError: advance,
    }).catch(() => null);
  } else {
    // youtube (and test fallback)
    await ytReady;
    state.player = await createYoutubeBackend('player', sourceId, position, {
      onEnded: advance,
      onError: advance,
    });
  }
};
