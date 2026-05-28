// YouTube IFrame API backend.
// Wraps YT.Player in the common player interface.

export function createYoutubeBackend(elementId, videoId, startSeconds, { onEnded, onError, onStateChange }) {
  // Create a fresh inner element so YT replaces it instead of #player itself,
  // keeping #player a stable <div> that Jellyfin can take over later.
  const container = document.getElementById(elementId);
  const target = document.createElement('div');
  container.replaceChildren(target);

  // Set when loadVideo() is called so onStateChange can trigger play once
  // the player emits UNSTARTED for the new video. Calling playVideo()
  // synchronously after loadVideoById() is unreliable when the player is
  // transitioning out of ENDED state.
  let pendingPlay = false;

  return new Promise(resolve => {
    const ytPlayer = new YT.Player(target, {
      videoId,
      playerVars: { start: Math.floor(startSeconds), autoplay: 1, controls: 0 },
      events: {
        onReady() {
          resolve({
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
            loadVideo(vid, start) {
              pendingPlay = true;
              ytPlayer.loadVideoById({ videoId: vid, startSeconds: Math.floor(start) });
            },
            destroy() { ytPlayer.destroy(); },
          });
        },
        onStateChange(event) {
          // UNSTARTED fires when loadVideoById begins loading a new video.
          // Call playVideo() here rather than synchronously after loadVideoById
          // so the player is in a valid state to accept the command.
          // This also covers iOS Safari where loadVideoById alone does not
          // autoplay in a cross-origin iframe outside a user gesture.
          if (event.data === YT.PlayerState.UNSTARTED && pendingPlay) {
            pendingPlay = false;
            ytPlayer.playVideo();
          }
          if      (event.data === YT.PlayerState.ENDED)   onEnded();
          else if (event.data === YT.PlayerState.PLAYING) onStateChange?.('playing');
          else if (event.data === YT.PlayerState.PAUSED)  onStateChange?.('paused');
        },
        onError() { onError(); },
      },
    });
  });
}
