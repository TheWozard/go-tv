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

const wrapper = document.getElementById('player-wrapper');
const advanceRetryMs = parseInt(wrapper?.dataset.advanceRetryMs, 10) || 2000;
const progressRateMs = parseInt(wrapper?.dataset.progressRateMs, 10) || 10000;

// advance is throttled so rapid onEnded/onError fires collapse into one call.
// Call advance.cancel() from controls to reset the window after user interaction.
const advance = throttle(async () => {
  try {
    const seconds = state.player ? state.player.getCurrentTime() : 0;
    await postNext(state.currentSource, seconds);
    const el = document.getElementById('player-state');
    if (el) applyStateFromEl(el);
  } catch {
    state.player?.pause();
  }
}, advanceRetryMs, { leading: true, trailing: false });

// reportProgress throttles progress updates to progressRateMs while playing.
// Called each rAF tick; trailing fires the last known position in each window.
const reportProgress = throttle(() => {
  const t = playingTime();
  if (t !== null) postProgress(state.currentSource, t);
}, progressRateMs, { leading: false, trailing: true });

// applyState syncs the player to server state. If the source hasn't changed
// it only seeks when positions are more than 5s apart (avoids seek loops).
function applyState(sourceKind, sourceId, seconds, stopSeconds, streamURL) {
  state.currentStop = stopSeconds;

  const sameSource = state.currentSource?.kind === sourceKind && state.currentSource?.id === sourceId;
  const sameKind = state.currentSource?.kind === sourceKind;
  state.currentSource = { kind: sourceKind, id: sourceId };

  if (sameSource) {
    if (state.player && Math.abs(state.player.getCurrentTime() - seconds) > 5) {
      state.player.seekTo(seconds);
    }
    return;
  }

  if (!state.player) return;

  if (sourceKind === 'youtube') {
    if (sameKind) {
      state.player.loadVideo(sourceId, seconds);
    } else {
      // Switching from Jellyfin → YouTube: create a new YouTube backend.
      ytReady.then(() => createYoutubeBackend('player', sourceId, seconds, { onEnded: advance, onError: advance }))
        .then(backend => { state.player = backend; });
    }
  } else if (sourceKind === 'jellyfin') {
    // Always re-init for Jellyfin (new item = new stream URL).
    createJellyfinBackend('player', streamURL, seconds, { onEnded: advance, onError: advance })
      .then(backend => { state.player = backend; })
      .catch(() => advance());
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

document.addEventListener('DOMContentLoaded', () => {
  const el = document.getElementById('player-state');
  if (el) applyStateFromEl(el);

  initControls(state, advance, reportProgress);
});

// ---- YouTube IFrame API bootstrap ----

window.onYouTubeIframeAPIReady = ytApiLoaded;

// startHere is called by the "Watch Here" button.
window.startHere = async function () {
  document.getElementById('start-screen').style.display = 'none';
  document.getElementById('player-wrapper').style.display = 'block';

  const el = document.getElementById('player-state');
  const sourceKind = el?.dataset.sourceKind || '';
  const sourceId = el?.dataset.sourceId || '';
  const streamURL = el?.dataset.streamUrl || '';
  const position = parseFloat(el?.dataset.position) || 0;
  const stopAt = parseFloat(el?.dataset.stopAt) || 0;

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
