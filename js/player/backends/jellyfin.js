// Jellyfin HLS backend using HLS.js.
// Plays the HLS master playlist URL from data-stream-url via a <video> element.

import Hls from 'hls.js';

export function createJellyfinBackend(elementId, streamURL, startSeconds, { onEnded, onError }) {
  const container = document.getElementById(elementId);
  const video = document.createElement('video');
  video.autoplay = true;
  container.replaceChildren(video);

  return new Promise((resolve, reject) => {
    function setup() {
      video.currentTime = startSeconds;
      video.play().catch(() => {});
      video.addEventListener('ended', onEnded);
      video.addEventListener('error', onError);
      resolve(wrap(video));
    }

    if (Hls.isSupported()) {
      const hls = new Hls();
      hls.loadSource(streamURL);
      hls.attachMedia(video);
      hls.on(Hls.Events.MANIFEST_PARSED, setup);
      hls.on(Hls.Events.ERROR, (_, data) => { if (data.fatal) onError(); });
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      // Safari native HLS
      video.src = streamURL;
      video.addEventListener('loadedmetadata', setup, { once: true });
    } else {
      reject(new Error('HLS not supported'));
    }
  });
}

function wrap(video) {
  return {
    play()           { video.play().catch(() => {}); },
    pause()          { video.pause(); },
    seekTo(s)        { video.currentTime = s; },
    getCurrentTime() { return video.currentTime; },
    getState() {
      if (video.ended)  return 'ended';
      if (video.paused) return 'paused';
      return 'playing';
    },
    loadVideo(_videoId, startSeconds) {
      // For Jellyfin the stream URL encodes the item; a source change in
      // applyState will construct a new backend instead of calling loadVideo.
      video.currentTime = startSeconds;
      video.play().catch(() => {});
    },
  };
}
