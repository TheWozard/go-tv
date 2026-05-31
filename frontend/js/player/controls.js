import { iconRewind, iconFastForward, iconCaptions, iconCaptionsOff } from '../icons.js';

// initControls wires up the overlay and keyboard shortcuts.
// state is a shared object { player, currentStop } mutated by player.js.
// advance() is called when playback should move to the next video.
export function initControls(state, advance, reportProgress, goToPrev, goToNext) {
  const overlay = document.getElementById('overlay');
  const progressWrap = document.getElementById('progress-bar-wrap');
  const progressFill = document.getElementById('progress-bar-fill');
  const progressStop = document.getElementById('progress-bar-stop');
  const skipMs = parseInt(document.getElementById('player-wrapper')?.dataset.skipIntervalMs, 10) || 10_000;

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
      if (cur > 0) state.lastTime = cur;
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

  // Supplement rAF with setInterval so stop is enforced in background tabs
  // where rAF is throttled. advance() is throttled so duplicate calls are safe.
  setInterval(() => {
    if (state.player && state.currentStop > 0) {
      if (state.player.getCurrentTime() >= state.currentStop) advance();
    }
  }, 500);

  const skipLeft    = document.getElementById('skip-left');
  const skipRight   = document.getElementById('skip-right');
  const seekBackBtn = document.getElementById('seek-back-btn');
  const seekFwdBtn  = document.getElementById('seek-fwd-btn');
  const skipSecs    = Math.round(skipMs / 1000);
  if (skipLeft)    skipLeft.textContent    = `« ${skipSecs}s`;
  if (skipRight)   skipRight.textContent   = `${skipSecs}s »`;
  if (seekBackBtn) seekBackBtn.innerHTML = `${iconRewind()} ${skipSecs}s`;
  if (seekFwdBtn)  seekFwdBtn.innerHTML  = `${skipSecs}s ${iconFastForward()}`;

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

  const prevEpBtn = document.getElementById('prev-ep-btn');
  const nextEpBtn = document.getElementById('next-ep-btn');
  const ccBtn     = document.getElementById('cc-btn');

  seekBackBtn?.addEventListener('click', e => { e.stopPropagation(); doSkip(-1); });
  seekFwdBtn?.addEventListener('click',  e => { e.stopPropagation(); doSkip(1); });
  prevEpBtn?.addEventListener('click',   e => { e.stopPropagation(); goToPrev?.(); });
  nextEpBtn?.addEventListener('click',   e => { e.stopPropagation(); goToNext?.(); });
  ccBtn?.addEventListener('click', e => {
    e.stopPropagation();
    if (!state.player?.toggleCC) return;
    const enabled = state.player.toggleCC();
    ccBtn.innerHTML = enabled ? iconCaptions() : iconCaptionsOff();
    ccBtn.classList.toggle('active', enabled);
  });

  // Double-tap left/right halves of the overlay to skip; single tap to play/pause.
  // touchHandled blocks the synthetic click that browsers fire after touchend.
  let touchHandled = false;
  let lastTouchTime = 0;
  let touchTapTimer = null;

  overlay.addEventListener('touchend', e => {
    if (e.target.closest('#progress-bar-wrap') || e.target.closest('#fullscreen-btn') || e.target.closest('#edit-btn') || e.target.closest('.seek-btn') || e.target.closest('.ep-btn')) return;
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
    if (e.target.closest('#edit-btn')) return;
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

  return {
    updatePauseState(s) {
      overlay.classList.toggle('paused', s === 'paused');
    },
    resetCC() {
      if (ccBtn) { ccBtn.innerHTML = iconCaptionsOff(); ccBtn.classList.remove('active'); }
    },
  };
}
