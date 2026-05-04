/* global htmx */

function sourceParams(source, seconds) {
  return { source_kind: source.kind, source_id: source.id, seconds: String(seconds) };
}

// postProgress reports the current playback position to the server.
// Called on each tick while playing; fire-and-forget.
export function postProgress(source, seconds) {
  htmx.ajax('POST', '/api/progress', { values: sourceParams(source, seconds) });
}

// postNext asks the server to advance past source and swaps in the new
// #player-state fragment via htmx. Returns a Promise that resolves when done.
export function postNext(source, seconds) {
  return htmx.ajax('POST', '/api/next', {
    target: '#player-state',
    swap: 'outerHTML',
    values: sourceParams(source, seconds),
  });
}

// getState fetches the current server state and swaps in a new #player-state
// fragment. Used to resync the UI after the player has been paused or hidden.
export function getState() {
  return htmx.ajax('GET', '/api/state', { target: '#player-state', swap: 'outerHTML' });
}
