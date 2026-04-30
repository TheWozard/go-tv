// postProgress reports the current playback position to the server.
// Called on each tick while playing; fire-and-forget.
export function postProgress(source, seconds) {
  fetch('/api/progress', {
    method: 'POST',
    body: new URLSearchParams({ source_kind: source.kind, source_id: source.id, seconds }),
  }).catch(() => {});
}

// postNext asks the server to advance past source and swaps in the new
// #player-state fragment. The server uses source_id to guard against
// double-advances, so concurrent calls with the same ID are safe.
export async function postNext(source, seconds) {
  await htmx.ajax('POST', '/api/next', {
    target: '#player-state',
    swap: 'outerHTML',
    values: { source_kind: source.kind, source_id: source.id, seconds: String(seconds) },
  });
}
