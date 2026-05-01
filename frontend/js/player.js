import { throttle } from 'lodash-es';
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

const advanceRetryMs = parseInt(document.getElementById('player-wrapper')?.dataset.advanceRetryMs, 10) || 2000;
const progressRateMs = parseInt(document.getElementById('player-wrapper')?.dataset.progressRateMs, 10) || 10000;

// advance is throttled so rapid onEnded/onError fires collapse into one call.
// Call advance.cancel() from controls to reset the window after user interaction.
// State is applied via the htmx:afterSwap listener; errors are handled by htmx:responseError.
const advance = throttle(() => {
  postNext(state.currentSource, state.player?.getCurrentTime() ?? 0);
}, advanceRetryMs, { leading: true, trailing: false });

// reportProgress throttles progress updates to progressRateMs while playing.
// Called each rAF tick; trailing fires the last known position in each window.
const reportProgress = throttle(() => {
  const t = playingTime();
  if (t !== null) postProgress(state.currentSource, t);
}, progressRateMs, { leading: false, trailing: true });

// loadToken increments on every source change so stale in-flight loads are discarded.
let loadToken = 0;

// applyState syncs the player to server state. On source change it immediately
// destroys the outgoing backend and nulls state.player before async construction,
// so the rAF loop and advance() see no player during the transition.
function applyState(sourceKind, sourceId, seconds, stopSeconds, streamURL) {
  state.currentStop = stopSeconds;

  const sameSource = state.currentSource?.kind === sourceKind && state.currentSource?.id === sourceId;
  const sameKind   = state.currentSource?.kind === sourceKind;
  state.currentSource = { kind: sourceKind, id: sourceId };

  if (sameSource) {
    if (state.player && Math.abs(state.player.getCurrentTime() - seconds) > 5) {
      state.player.seekTo(seconds);
    }
    return;
  }

  // Source changed — cancel any pending advance and invalidate in-flight loads.
  advance.cancel();
  const token = ++loadToken;

  // YouTube → YouTube: reuse the existing player, no tear-down needed.
  if (sourceKind === 'youtube' && sameKind && state.player) {
    state.player.loadVideo(sourceId, seconds);
    return;
  }

  // Full backend swap: null state.player immediately so rAF/progress stop using it.
  const outgoing = state.player;
  state.player = null;
  outgoing?.destroy();

  if (sourceKind === 'youtube') {
    ytReady
      .then(() => createYoutubeBackend('player', sourceId, seconds, { onEnded: advance, onError: advance }))
      .then(backend => {
        if (token !== loadToken) { backend.destroy(); return; }
        state.player = backend;
      });
  } else if (sourceKind === 'jellyfin') {
    createJellyfinBackend('player', streamURL, seconds, { onEnded: advance, onError: advance })
      .then(backend => {
        if (token !== loadToken) { backend.destroy(); return; }
        state.player = backend;
      })
      .catch(() => { if (token === loadToken) advance(); });
  }
}

// ---- State observer ----

function applyCurrentState() {
  const el = document.getElementById('player-state');
  if (!el) return;
  applyState(
    el.dataset.sourceKind,
    el.dataset.sourceId,
    parseFloat(el.dataset.position) || 0,
    parseFloat(el.dataset.stopAt) || 0,
    el.dataset.streamUrl || '',
  );
}

document.addEventListener('DOMContentLoaded', () => {
  initControls(state, advance, reportProgress);

  // Apply player state after htmx swaps in a new #player-state fragment.
  document.addEventListener('htmx:afterSwap', e => {
    if (e.detail.target?.id === 'player-state') applyCurrentState();
  });

  // htmx.ajax doesn't reject on HTTP errors — pause the player when /api/next fails.
  document.addEventListener('htmx:responseError', e => {
    if (e.detail.target?.id === 'player-state') state.player?.pause();
  });
});

// ---- YouTube IFrame API bootstrap ----

window.onYouTubeIframeAPIReady = ytApiLoaded;

// startHere is called by the "Watch Here" button.
window.startHere = function () {
  document.getElementById('start-screen').style.display = 'none';
  document.getElementById('player-wrapper').style.display = 'block';
  applyCurrentState();
};
