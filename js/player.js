import { throttle } from 'lodash-es';

let player;
let resolveYtReady;
const ytReady = new Promise(resolve => { resolveYtReady = resolve; });

let currentSource = null; // { kind, id }
let currentStop = 0;

const TICK_MS = 1000;

// ---- API calls ----

const postProgress = throttle((source, seconds) => {
  fetch('/api/progress', {
    method: 'POST',
    body: new URLSearchParams({ source_kind: source.kind, source_id: source.id, seconds }),
  }).catch(() => {});
}, 5_000);

const postNext = throttle(async (source, seconds) => {
  await htmx.ajax('POST', '/api/next', {
    target: '#player-state',
    swap: 'outerHTML',
    values: { source_kind: source.kind, source_id: source.id, seconds: String(seconds) },
  });
}, 30_000, { leading: true, trailing: false });

// ---- Playback control ----

function advance() {
  const seconds = (player && typeof player.getCurrentTime === 'function')
    ? player.getCurrentTime() : 0;
  postNext(currentSource, seconds);
}

function applyState(sourceKind, sourceId, seconds, stopSeconds) {
  currentStop = stopSeconds;
  if (currentSource?.kind === sourceKind && currentSource?.id === sourceId) {
    if (player && Math.abs(player.getCurrentTime() - seconds) > 5) {
      player.seekTo(seconds, true);
    }
    return;
  }
  currentSource = { kind: sourceKind, id: sourceId };
  if (sourceKind === 'youtube' && player) {
    player.loadVideoById({ videoId: sourceId, startSeconds: Math.floor(seconds) });
  }
}

// ---- State observer ----

function applyStateFromEl(el) {
  applyState(
    el.dataset.sourceKind,
    el.dataset.sourceId,
    parseFloat(el.dataset.position) || 0,
    parseFloat(el.dataset.stopAt) || 0,
  );
}

document.addEventListener('htmx:afterSwap', e => {
  if (e.detail.target?.id !== 'player-state') return;
  const el = document.getElementById('player-state');
  if (el) applyStateFromEl(el);
});

document.addEventListener('DOMContentLoaded', () => {
  const el = document.getElementById('player-state');
  if (el) applyStateFromEl(el);

  // ---- Overlay: cursor hide + click to pause/play ----
  const overlay = document.getElementById('overlay');
  let cursorTimer;
  overlay.addEventListener('mousemove', () => {
    overlay.style.cursor = 'default';
    clearTimeout(cursorTimer);
    cursorTimer = setTimeout(() => { overlay.style.cursor = 'none'; }, 3000);
  });
  function togglePlayPause() {
    if (!player || typeof player.getPlayerState !== 'function') return;
    if (player.getPlayerState() === YT.PlayerState.PLAYING) {
      player.pauseVideo();
    } else {
      player.playVideo();
    }
  }

  overlay.addEventListener('click', togglePlayPause);
  document.addEventListener('keydown', e => {
    if (e.code === 'Space' && e.target === document.body) {
      e.preventDefault();
      togglePlayPause();
    }
  });
});

// ---- Tick loop ----

setInterval(() => {
  if (!player || typeof player.getCurrentTime !== 'function') return;
  if (player.getPlayerState() !== YT.PlayerState.PLAYING) return;

  const t = player.getCurrentTime();
  postProgress(currentSource, t);
  if (currentStop > 0 && t >= currentStop) advance();
}, TICK_MS);

// ---- YouTube IFrame API bootstrap ----

window.onYouTubeIframeAPIReady = function () {
  resolveYtReady();
};

window.startHere = async function () {
  document.getElementById('start-screen').style.display = 'none';
  document.getElementById('player-wrapper').style.display = 'block';

  await ytReady;

  const el = document.getElementById('player-state');
  const sourceKind = el?.dataset.sourceKind || '';
  const sourceId = el?.dataset.sourceId || '';
  const position = parseFloat(el?.dataset.position) || 0;
  const stopAt = parseFloat(el?.dataset.stopAt) || 0;

  currentSource = { kind: sourceKind, id: sourceId };
  currentStop = stopAt;

  player = new YT.Player('player', {
    videoId: sourceId,
    playerVars: { start: Math.floor(position), autoplay: 1, controls: 0 },
    events: {
      onStateChange(event) {
        if (event.data === YT.PlayerState.ENDED) advance();
      },
      onError() { advance(); },
    },
  });
};
