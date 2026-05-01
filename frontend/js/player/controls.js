import { debounce } from 'lodash-es';

// initControls wires up the overlay and keyboard shortcuts.
// state is a shared object { player, currentStop } mutated by player.js.
// advance() is called when playback should move to the next video.
export function initControls(state, advance, reportProgress) {
  const overlay = document.getElementById('overlay');
  const progressWrap = document.getElementById('progress-bar-wrap');
  const progressFill = document.getElementById('progress-bar-fill');
  const progressStop = document.getElementById('progress-bar-stop');
  const fullscreenBtn = document.getElementById('fullscreen-btn');
  const skipMs = parseInt(document.getElementById('player-wrapper')?.dataset.skipIntervalMs, 10) || 10_000;

  // Control visibility (cursor, progress bar, fullscreen button).
  const HIDE_DELAY_MS = 3000;

  function showControls() {
    overlay.style.cursor = 'default';
    progressWrap?.classList.add('visible');
    fullscreenBtn?.classList.add('visible');
  }

  const hideControls = debounce(() => {
    overlay.style.cursor = 'none';
    progressWrap?.classList.remove('visible');
    fullscreenBtn?.classList.remove('visible');
  }, HIDE_DELAY_MS);

  overlay.addEventListener('mousemove', () => {
    showControls();
    hideControls();
  });

  // Show controls on touch so they're visible before the tap action fires.
  overlay.addEventListener('touchstart', () => {
    showControls();
    hideControls();
  }, { passive: true });

  // Fullscreen toggle.
  function toggleFullscreen() {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  }

  fullscreenBtn?.addEventListener('click', e => {
    e.stopPropagation();
    toggleFullscreen();
  });

  document.addEventListener('fullscreenchange', () => {
    const isFs = !!document.fullscreenElement;
    const enter = document.getElementById('fs-enter');
    const exit  = document.getElementById('fs-exit');
    if (enter) enter.style.display = isFs ? 'none' : '';
    if (exit)  exit.style.display  = isFs ? '' : 'none';
  });

  // Seek to a position based on a clientX coordinate over the progress bar.
  function seekFromX(clientX) {
    if (!state.player) return;
    const dur = state.player.getDuration();
    if (!dur) return;
    const rect = progressWrap.getBoundingClientRect();
    const pct  = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
    state.player.seekTo(pct * dur);
    advance.cancel();
  }

  // Click-to-seek on the progress bar (stop propagation to avoid toggling play/pause).
  progressWrap?.addEventListener('click', e => {
    e.stopPropagation();
    seekFromX(e.clientX);
  });

  // Touch-to-seek on the progress bar (passive: false so preventDefault works).
  progressWrap?.addEventListener('touchstart', e => {
    e.stopPropagation();
    e.preventDefault();
    seekFromX(e.touches[0].clientX);
  }, { passive: false });

  progressWrap?.addEventListener('touchmove', e => {
    e.stopPropagation();
    e.preventDefault();
    seekFromX(e.touches[0].clientX);
  }, { passive: false });

  // rAF loop: update progress bar and check stop time at frame rate for accuracy.
  function tickProgress() {
    if (state.player) {
      const cur = state.player.getCurrentTime();
      const dur = state.player.getDuration();
      if (dur > 0) {
        if (progressFill) {
          progressFill.style.width = ((cur / dur) * 100).toFixed(2) + '%';
        }
        if (progressStop) {
          if (state.currentStop > 0) {
            progressStop.style.left = (Math.min(1, state.currentStop / dur) * 100).toFixed(2) + '%';
            progressStop.classList.add('active');
          } else {
            progressStop.classList.remove('active');
          }
        }
      }
      if (state.currentStop > 0 && cur >= state.currentStop) {
        advance();
      }
      reportProgress();
    }
    requestAnimationFrame(tickProgress);
  }
  requestAnimationFrame(tickProgress);

  const skipLeft  = document.getElementById('skip-left');
  const skipRight = document.getElementById('skip-right');
  const skipSecs  = Math.round(skipMs / 1000);
  if (skipLeft)  skipLeft.textContent  = `« ${skipSecs}s`;
  if (skipRight) skipRight.textContent = `${skipSecs}s »`;

  function flashSkip(dir) {
    const el = dir > 0 ? skipRight : skipLeft;
    if (!el) return;
    el.classList.remove('active');
    void el.offsetWidth;
    el.classList.add('active');
  }

  function doSkip(dir) {
    if (!state.player) return;
    const newTime = state.player.getCurrentTime() + dir * (skipMs / 1000);
    state.player.seekTo(newTime);
    advance.cancel();
    if (dir > 0 && state.currentStop > 0 && newTime >= state.currentStop) advance();
    flashSkip(dir);
  }

  function togglePlayPause() {
    if (!state.player) return;
    if (state.player.getState() === 'playing') {
      state.player.pause();
    } else {
      state.player.play();
    }
  }

  // Double-tap left/right halves of the overlay to skip; single tap to play/pause.
  // touchHandled blocks the synthetic click that browsers fire after touchend.
  let touchHandled = false;
  let lastTouchTime = 0;
  let touchTapTimer = null;

  overlay.addEventListener('touchend', e => {
    if (e.target.closest('#progress-bar-wrap') || e.target.closest('#fullscreen-btn')) return;
    touchHandled = true;

    const now   = Date.now();
    const touch = e.changedTouches[0];

    if (now - lastTouchTime < 300) {
      clearTimeout(touchTapTimer);
      touchTapTimer  = null;
      lastTouchTime  = 0;
      doSkip(touch.clientX > window.innerWidth / 2 ? 1 : -1);
    } else {
      lastTouchTime = now;
      touchTapTimer = setTimeout(() => {
        touchTapTimer = null;
        togglePlayPause();
      }, 300);
    }
  });

  // Mouse clicks — skip the event if touch already handled it.
  overlay.addEventListener('click', e => {
    if (touchHandled) { touchHandled = false; return; }
    togglePlayPause();
  });

  // Space: toggle play/pause.
  // ArrowRight/Left: seek by skip_interval (data-skip-interval-ms on #player-wrapper).
  //   A forward jump past currentStop triggers advance() immediately.
  document.addEventListener('keydown', e => {
    if (e.target !== document.body) return;
    if (e.code === 'Space') {
      e.preventDefault();
      togglePlayPause();
    } else if (e.code === 'ArrowRight' || e.code === 'ArrowLeft') {
      e.preventDefault();
      if (!state.player) return;
      const dir = e.code === 'ArrowRight' ? 1 : -1;
      const newTime = state.player.getCurrentTime() + dir * (skipMs / 1000);
      state.player.seekTo(newTime);
      advance.cancel();
      if (dir > 0 && state.currentStop > 0 && newTime >= state.currentStop) advance();
    }
  });
}
