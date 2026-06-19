// Jellyfin HLS backend using HLS.js.
// Plays the HLS master playlist URL from data-stream-url via a <video> element.

import Hls from 'hls.js';

export function createJellyfinBackend(elementId, streamURL, startSeconds, stopSeconds, { onEnded, onError, onStateChange }) {
  const container = document.getElementById(elementId);
  const video = document.createElement('video');
  container.replaceChildren(video);

  return new Promise((resolve, reject) => {
    function setup(hls) {
      const onPlay       = () => onStateChange?.('playing');
      const onPause      = () => onStateChange?.('paused');
      const onVideoError = () => { console.error('[jellyfin] video error', video.error); onError(); };
      // The overlay sits on top with pointer-events:none while playing, so clicks
      // fall through to the <video> element. Toggle pause/play here so the overlay
      // becomes interactive (pointer-events:auto) once the player pauses.
      const onVideoClick = () => { if (video.paused) video.play().catch(() => {}); else video.pause(); };
      video.addEventListener('ended', onEnded);
      video.addEventListener('error', onVideoError);
      video.addEventListener('play',  onPlay);
      video.addEventListener('pause', onPause);
      video.addEventListener('click', onVideoClick);
      const backend = wrap(video, hls, stopSeconds, { onEnded, onError: onVideoError, onPlay, onPause, onVideoClick });
      video.currentTime = startSeconds;
      backend.play();
      resolve(backend);
    }

    if (Hls.isSupported()) {
      const hls = new Hls();
      hls.loadSource(streamURL);
      hls.attachMedia(video);
      hls.on(Hls.Events.MANIFEST_PARSED, () => setup(hls));
      hls.on(Hls.Events.ERROR, (_, data) => { if (data.fatal) { console.error('[jellyfin] hls fatal error', data.type, data.details, data); onError(); } });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Safari native HLS — no fragment-level control; stopSeconds enforced by controls.js.
      video.src = streamURL;
      video.addEventListener('loadedmetadata', () => setup(null), { once: true });
    } else {
      reject(new Error('HLS not supported'));
    }
  });
}

function wrap(video, hls, stopSeconds, { onEnded, onError, onPlay, onPause, onVideoClick }) {
  // Stop buffering fragments that start at or beyond the clip end.
  if (hls && stopSeconds > 0) {
    hls.on(Hls.Events.FRAG_LOADING, (_, data) => {
      if (data.frag.start >= stopSeconds) hls.stopLoad();
    });
  }

  const play = () => video.play().catch(() => {});
  return {
    play,
    pause()          { video.pause(); },
    seekTo(s)        { video.currentTime = s; hls?.startLoad(); },
    getCurrentTime() { return video.currentTime; },
    getDuration()    { return isFinite(video.duration) ? video.duration : 0; },
    getState() {
      if (video.ended)  return 'ended';
      if (video.paused) return 'paused';
      return 'playing';
    },
    loadVideo(_videoId, startSeconds) {
      // For Jellyfin the stream URL encodes the item; a source change in
      // applyState will construct a new backend instead of calling loadVideo.
      video.currentTime = startSeconds;
      hls?.startLoad();
      play();
    },
    destroy() {
      video.pause();
      video.removeEventListener('ended',  onEnded);
      video.removeEventListener('error',  onError);
      video.removeEventListener('play',   onPlay);
      video.removeEventListener('pause',  onPause);
      video.removeEventListener('click',  onVideoClick);
      hls?.destroy();
      video.src = '';
    },
  };
}
