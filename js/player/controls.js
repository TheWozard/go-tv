import { debounce } from 'lodash-es';

// initControls wires up the overlay and keyboard shortcuts.
// state is a shared object { player, currentStop } mutated by player.js.
// advance() is called when playback should move to the next video.
export function initControls(state, advance) {
  const overlay = document.getElementById('overlay');
  const hideCursor = debounce(() => { overlay.style.cursor = 'none'; }, 3000);
  overlay.addEventListener('mousemove', () => {
    overlay.style.cursor = 'default';
    hideCursor();
  });

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
      if (dir > 0 && state.currentStop > 0 && newTime >= state.currentStop) advance();
    }
  });
}
