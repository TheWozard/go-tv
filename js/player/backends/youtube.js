// YouTube IFrame API backend.
// Wraps YT.Player in the common player interface.

export function createYoutubeBackend(elementId, videoId, startSeconds, { onEnded, onError }) {
  // Create a fresh inner element so YT replaces it instead of #player itself,
  // keeping #player a stable <div> that Jellyfin can take over later.
  const container = document.getElementById(elementId);
  const target = document.createElement('div');
  container.replaceChildren(target);

  return new Promise(resolve => {
    const ytPlayer = new YT.Player(target, {
      videoId,
      playerVars: { start: Math.floor(startSeconds), autoplay: 1, controls: 0 },
      events: {
        onReady() { resolve(wrap(ytPlayer, { onEnded, onError })); },
        onStateChange(event) {
          if (event.data === YT.PlayerState.ENDED) onEnded();
        },
        onError() { onError(); },
      },
    });
  });
}

function wrap(ytPlayer, { onEnded, onError }) {
  return {
    play()               { ytPlayer.playVideo(); },
    pause()              { ytPlayer.pauseVideo(); },
    seekTo(s)            { ytPlayer.seekTo(s, true); },
    getCurrentTime()     { return ytPlayer.getCurrentTime(); },
    getDuration()        { return ytPlayer.getDuration() || 0; },
    getState() {
      switch (ytPlayer.getPlayerState()) {
        case YT.PlayerState.PLAYING: return 'playing';
        case YT.PlayerState.PAUSED:  return 'paused';
        case YT.PlayerState.ENDED:   return 'ended';
        default:                     return 'other';
      }
    },
    loadVideo(videoId, startSeconds) {
      ytPlayer.loadVideoById({ videoId, startSeconds: Math.floor(startSeconds) });
    },
  };
}
