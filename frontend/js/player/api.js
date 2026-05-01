// postProgress reports the current playback position to the server.
// Called on each tick while playing; fire-and-forget.
export function postProgress(source, seconds) {
  fetch('/api/progress', {
    method: 'POST',
    body: new URLSearchParams({ source_kind: source.kind, source_id: source.id, seconds }),
  }).catch(() => {});
}

// postNext asks the server to advance past source and swaps in the new
// #player-state fragment. Throws on network or HTTP errors.
export async function postNext(source, seconds) {
  const resp = await fetch('/api/next', {
    method: 'POST',
    body: new URLSearchParams({ source_kind: source.kind, source_id: source.id, seconds: String(seconds) }),
  });
  if (!resp.ok) throw new Error(`next: ${resp.status}`);
  const html = await resp.text();
  const target = document.getElementById('player-state');
  if (target) target.outerHTML = html;
}
