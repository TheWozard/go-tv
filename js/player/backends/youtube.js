// YouTube IFrame API backend.
// Wraps YT.Player in the common player interface.

export function createYoutubeBackend(elementId, videoId, startSeconds, { onEnded, onError }) {
  return new Promise(resolve => {
    const ytPlayer = new YT.Player(elementId, {
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
