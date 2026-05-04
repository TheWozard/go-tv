// Jellyfin HLS backend using HLS.js.
// Plays the HLS master playlist URL from data-stream-url via a <video> element.

import Hls from 'hls.js';

export function createJellyfinBackend(elementId, streamURL, startSeconds, { onEnded, onError, onStateChange }) {
  const container = document.getElementById(elementId);
  const video = document.createElement('video');
  container.replaceChildren(video);

  return new Promise((resolve, reject) => {
    function setup(hls) {
      const onPlay  = () => onStateChange?.('playing');
      const onPause = () => onStateChange?.('paused');
      video.addEventListener('ended', onEnded);
      video.addEventListener('error', onError);
      video.addEventListener('play',  onPlay);
      video.addEventListener('pause', onPause);
      const backend = wrap(video, hls, { onEnded, onError, onPlay, onPause });
      video.currentTime = startSeconds;
      backend.play();
      resolve(backend);
    }

    if (Hls.isSupported()) {
      const hls = new Hls();
      hls.loadSource(streamURL);
      hls.attachMedia(video);
      hls.on(Hls.Events.MANIFEST_PARSED, () => setup(hls));
      hls.on(Hls.Events.ERROR, (_, data) => { if (data.fatal) onError(); });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Safari native HLS
      video.src = streamURL;
      video.addEventListener('loadedmetadata', () => setup(null), { once: true });
    } else {
      reject(new Error('HLS not supported'));
    }
  });
}

function wrap(video, hls, { onEnded, onError, onPlay, onPause }) {
  const play = () => video.play().catch(() => {});
  return {
    play,
    pause()          { video.pause(); },
    seekTo(s)        { video.currentTime = s; },
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
      play();
    },
    destroy() {
      video.pause();
      video.removeEventListener('ended',  onEnded);
      video.removeEventListener('error',  onError);
      video.removeEventListener('play',   onPlay);
      video.removeEventListener('pause',  onPause);
      hls?.destroy();
      video.src = '';
    },
  };
}
