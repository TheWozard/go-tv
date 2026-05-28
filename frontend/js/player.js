import { throttle } from 'lodash-es';
import { postProgress, postNext, postNextEp, postPrevEp, getState } from './player/api.js';
import { initControls } from './player/controls.js';
import { createYoutubeBackend } from './player/backends/youtube.js';
import { createJellyfinBackend } from './player/backends/jellyfin.js';

// state is shared with controls.js by reference so mutations here are visible there.
const state = {
  player: null,        // backend instance ({ play, pause, seekTo, getCurrentTime, getState })
  currentSource: null, // { kind, id } of the loaded video
  currentStop: 0,      // server-imposed stop time in seconds (0 = no stop)
  lastTime: 0,         // last non-zero currentTime seen in rAF loop; fallback for platforms that reset to 0 on ENDED
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

const advanceRetryMs     = parseInt(document.getElementById('player-wrapper')?.dataset.advanceRetryMs, 10) || 2000;
const progressRateMs     = parseInt(document.getElementById('player-wrapper')?.dataset.progressRateMs, 10) || 10000;
const resyncThresholdMs  = parseInt(document.getElementById('player-wrapper')?.dataset.resyncThresholdMs, 10) || 60_000;

// advance is throttled so rapid onEnded/onError fires collapse into one call.
// Call advance.cancel() from controls to reset the window after user interaction.
// State is applied via the htmx:afterSwap listener; errors are handled by htmx:responseError.
const advance = throttle(() => {
  // Some platforms (iPad Chrome, Vivaldi) reset getCurrentTime() to 0 when ENDED fires.
  // Fall back to the last non-zero time seen in the rAF loop so postNext sends a valid position.
  const rawTime = state.player?.getCurrentTime();
  const t = rawTime || state.lastTime || 0;
  postNext(state.currentSource, t);
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
    if (state.player) {
      const ended = state.player.getState() === 'ended';
      if (ended || seconds - state.player.getCurrentTime() > 5) {
        // Cancel the throttle so the ENDED that fires after seeking can trigger advance().
        advance.cancel();
        state.player.seekTo(seconds);
      }
      if (ended) state.player.play();
    }
    return;
  }

  // Source changed — cancel any pending advance and invalidate in-flight loads.
  // Clear resync state; the new backend will re-arm it via onStateChange.
  advance.cancel();
  clearTimeout(pauseTimer);
  pauseTimer = null;
  needsResync = false;
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

  const callbacks = { onEnded: advance, onError: advance, onStateChange };

  if (sourceKind === 'youtube') {
    ytReady
      .then(() => createYoutubeBackend('player', sourceId, seconds, callbacks))
      .then(backend => {
        if (token !== loadToken) { backend.destroy(); return; }
        state.player = backend;
      });
  } else if (sourceKind === 'jellyfin') {
    createJellyfinBackend('player', streamURL, seconds, stopSeconds, callbacks)
      .then(backend => {
        if (token !== loadToken) { backend.destroy(); return; }
        state.player = backend;
      })
      .catch(() => { if (token === loadToken) advance(); });
  }
}

// ---- State observer ----

function applyCurrentState() {
  if (!started) return;
  const el = document.getElementById('player-state');
  if (!el) return;
  if (el.dataset.ended) {
    state.player?.destroy();
    state.player = null;
    state.currentSource = null;
    started = false;
    document.getElementById('start-screen').style.display = '';
    document.getElementById('player-wrapper').style.display = '';
    return;
  }
  applyState(
    el.dataset.sourceKind,
    el.dataset.sourceId,
    parseFloat(el.dataset.position) || 0,
    parseFloat(el.dataset.stopAt) || 0,
    el.dataset.streamUrl || '',
  );
}

// ---- Resync after UI pause ----
//
// Calls getState() to fetch the current backend position and swap in a new
// #player-state fragment. applyCurrentState() fires via htmx:afterSwap and
// seeks / switches source as needed.

let controls    = null;  // return value of initControls
let started     = false; // true once the user has clicked "Watch Here"
let hiddenAt    = 0;     // timestamp when page was last hidden
let pauseTimer  = null;  // setTimeout handle; sets needsResync after threshold
let needsResync = false; // true once pause threshold is reached; acted on next play

function resync() {
  needsResync = false;
  hiddenAt = 0;
  clearTimeout(pauseTimer);
  pauseTimer = null;
  getState();
}

// Called by each backend when playback state changes to 'playing' or 'paused'.
function onStateChange(s) {
  controls?.updatePauseState(s);
  if (s === 'playing') {
    clearTimeout(pauseTimer);
    pauseTimer = null;
    if (needsResync) resync();
  } else if (pauseTimer === null && !needsResync) {
    pauseTimer = setTimeout(() => {
      pauseTimer = null;
      needsResync = true;
    }, resyncThresholdMs);
  }
}

// Tab hidden/visible: resync immediately on restore if hidden longer than the
// threshold. The video may already be playing so we can't wait for un-pause.
// (Background tabs have throttled timers, so we check elapsed time on restore.)
document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    hiddenAt = Date.now();
  } else if (hiddenAt > 0) {
    if (Date.now() - hiddenAt >= resyncThresholdMs) resync();
    hiddenAt = 0;
  }
});

function goToNext() {
  advance.cancel();
  postNextEp(state.currentSource);
}

function goToPrev() {
  advance.cancel();
  postPrevEp(state.currentSource);
}

document.addEventListener('DOMContentLoaded', () => {
  controls = initControls(state, advance, reportProgress, goToPrev, goToNext);

  // Apply player state after htmx swaps in a new #player-state fragment.
  document.addEventListener('htmx:afterSwap', e => {
    if (e.detail.target?.id === 'player-state') applyCurrentState();
  });

  // htmx.ajax doesn't reject on HTTP errors — pause the player on unexpected errors.
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
  started = true;
  applyCurrentState();
};
