import { debounce } from 'lodash-es';

// initControls wires up the overlay and keyboard shortcuts.
// state is a shared object { player, currentStop } mutated by player.js.
// advance() is called when playback should move to the next video.
export function initControls(state, advance, reportProgress) {
  const overlay = document.getElementById('overlay');
  const progressWrap = document.getElementById('progress-bar-wrap');
  const progressFill = document.getElementById('progress-bar-fill');
  const progressStop = document.getElementById('progress-bar-stop');

  // Cursor + progress bar visibility on mouse move.
  const hideCursor   = debounce(() => { overlay.style.cursor = 'none'; }, 3000);
  const hideProgress = debounce(() => progressWrap?.classList.remove('visible'), 3000);
  overlay.addEventListener('mousemove', () => {
    overlay.style.cursor = 'default';
    hideCursor();
    progressWrap?.classList.add('visible');
    hideProgress();
  });

  // Click-to-seek on the progress bar (stop propagation to avoid toggling play/pause).
  progressWrap?.addEventListener('click', e => {
    e.stopPropagation();
    if (!state.player) return;
    const dur = state.player.getDuration?.() ?? 0;
    if (!dur) return;
    const rect = progressWrap.getBoundingClientRect();
    const pct  = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    state.player.seekTo(pct * dur);
    advance.cancel();
  });

  // rAF loop: update progress bar and check stop time at frame rate for accuracy.
  function tickProgress() {
    if (state.player) {
      const cur = state.player.getCurrentTime();
      const dur = state.player.getDuration?.() ?? 0;
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

  function togglePlayPause() {
    if (!state.player) return;
    if (state.player.getState() === 'playing') {
      state.player.pause();
    } else {
      state.player.play();
    }
  }

  overlay.addEventListener('click', togglePlayPause);

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
      const skipMs = parseInt(document.getElementById('player-wrapper')?.dataset.skipIntervalMs, 10) || 10_000;
      const dir = e.code === 'ArrowRight' ? 1 : -1;
      const newTime = state.player.getCurrentTime() + dir * (skipMs / 1000);
      state.player.seekTo(newTime);
      advance.cancel();
      if (dir > 0 && state.currentStop > 0 && newTime >= state.currentStop) advance();
    }
  });
}
