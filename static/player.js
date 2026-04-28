/**
 * go-tv player bridge
 *
 * Connects the YouTube IFrame API to the Datastar SSE state stream.
 * State is pushed from the server as a #player-state fragment with
 * data-video-id, data-seconds, and data-stop-seconds attributes.
 * A MutationObserver watches for attribute changes and drives the player.
 */

let player;
let resolveYtReady;
const ytReady = new Promise(resolve => { resolveYtReady = resolve; });

let currentVideoId;
let currentStop = 0;

const TICK_MS = 1000;
const PROGRESS_EVERY_N_TICKS = 5;
let advancing = false;

// ---- API calls ----

async function postProgress(videoId, seconds) {
  await fetch('/api/progress', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ video_id: videoId, seconds }),
  });
}

async function postNext(videoId, seconds) {
  const res = await fetch('/api/next', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ video_id: videoId, seconds }),
  });
  if (!res.ok) throw new Error('failed to advance');
}

// ---- Playback control ----

async function advance() {
  if (advancing) return;
  advancing = true;
  try {
    const seconds = (player && typeof player.getCurrentTime === 'function')
      ? player.getCurrentTime() : 0;
    await postNext(currentVideoId, seconds);
    // State update arrives via SSE -> MutationObserver -> applyState
  } catch {
    player?.pauseVideo();
  } finally {
    advancing = false;
  }
}

function applyState(videoId, seconds, stopSeconds) {
  currentStop = stopSeconds;
  if (currentVideoId === videoId) {
    if (player && Math.abs(player.getCurrentTime() - seconds) > 5) {
      player.seekTo(seconds, true);
    }
    return;
  }
  currentVideoId = videoId;
  if (player) {
    player.loadVideoById({ videoId, startSeconds: Math.floor(seconds) });
  }
}

// ---- SSE state observer ----

// Watch #player-state for attribute changes pushed by Datastar
const stateObserver = new MutationObserver(() => {
  const el = document.getElementById('player-state');
  if (!el) return;
  applyState(
    el.dataset.videoId,
    parseFloat(el.dataset.seconds) || 0,
    parseFloat(el.dataset.stopSeconds) || 0,
  );
});

document.addEventListener('DOMContentLoaded', () => {
  const el = document.getElementById('player-state');
  if (el) {
    stateObserver.observe(el, { attributes: true });
    // Apply initial server-rendered state immediately
    applyState(
      el.dataset.videoId,
      parseFloat(el.dataset.seconds) || 0,
      parseFloat(el.dataset.stopSeconds) || 0,
    );
  }
});

// ---- Tick loop ----

let tickCount = 0;
setInterval(() => {
  if (!player || typeof player.getCurrentTime !== 'function') return;
  if (player.getPlayerState() !== YT.PlayerState.PLAYING) return;

  const t = player.getCurrentTime();
  if (++tickCount % PROGRESS_EVERY_N_TICKS === 0) {
    postProgress(currentVideoId, t).catch(() => {});
  }
  if (currentStop > 0 && t >= currentStop) advance();
}, TICK_MS);

// ---- Edit link show/hide ----

const editLink = document.getElementById('edit-link');
let hideTimer;
let catchOverlay = null;

function showEdit() {
  if (catchOverlay) { catchOverlay.remove(); catchOverlay = null; }
  editLink.classList.add('visible');
  clearTimeout(hideTimer);
  hideTimer = setTimeout(hideEdit, 3000);
}

function hideEdit() {
  editLink.classList.remove('visible');
  catchOverlay = document.createElement('div');
  catchOverlay.style.cssText = 'position:fixed;inset:0;z-index:9;';
  catchOverlay.addEventListener('mousemove', showEdit, { once: true });
  document.body.appendChild(catchOverlay);
}

document.addEventListener('mousemove', showEdit);

// ---- YouTube IFrame API bootstrap ----

window.onYouTubeIframeAPIReady = function () {
  resolveYtReady();
};

window.startHere = async function () {
  document.getElementById('start-screen').style.display = 'none';
  document.getElementById('player-wrapper').style.display = 'block';

  await ytReady;

  const el = document.getElementById('player-state');
  const videoId = el?.dataset.videoId || '';
  const seconds = parseFloat(el?.dataset.seconds) || 0;
  const stopSeconds = parseFloat(el?.dataset.stopSeconds) || 0;

  currentVideoId = videoId;
  currentStop = stopSeconds;

  player = new YT.Player('player', {
    videoId,
    playerVars: { start: Math.floor(seconds), autoplay: 1 },
    events: {
      onStateChange(event) {
        if (event.data === YT.PlayerState.ENDED) advance();
      },
      onError() { advance(); },
    },
  });
};
