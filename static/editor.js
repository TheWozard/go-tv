// Position slider visual feedback
function updatePosFill(input) {
  const wrap = input.closest('.pos-slider-wrap');
  const fill = wrap.querySelector('.pos-fill');
  const max = parseFloat(input.max) || 1;
  fill.style.width = (parseFloat(input.value) / max * 100) + '%';
  const display = wrap.closest('.pos-wrap')?.querySelector('.pos-display');
  if (display) display.textContent = fmt(parseFloat(input.value));
}

function jumpTo(videoId, seconds) {
  fetch('/api/jump', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ video_id: videoId, seconds: parseFloat(seconds) }),
  }).catch(() => {});
}

function fmt(secs) {
  const s = Math.round(secs);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// SponsorBlock panel toggle
function closeSB(videoId) {
  const panel = document.getElementById('sb-panel-' + videoId);
  if (panel) panel.innerHTML = '';
  const card = document.querySelector(`.card[data-id="${CSS.escape(videoId)}"]`);
  if (card) card.querySelector('.sb-btn')?.classList.remove('open');
}

function toggleSBCheck(idx, row) {
  const cb = row.querySelector('.sb-check');
  if (cb) cb.checked = !cb.checked;
}

// Collect checked SponsorBlock segments and POST them; response is SSE that patches the card
document.addEventListener('click', async e => {
  const btn = e.target.closest('.sb-apply-btn');
  if (!btn) return;
  const panel = btn.closest('.sb-panel');
  const card = btn.closest('.card');
  if (!panel || !card) return;
  const videoId = card.dataset.id;
  const checks = [...panel.querySelectorAll('.sb-check:checked')];
  const allSegs = [...panel.querySelectorAll('.sb-seg-row')].map(row => ({
    index: parseInt(row.dataset.idx),
    start: parseFloat(row.querySelector('.sb-time').dataset.start),
    end: parseFloat(row.querySelector('.sb-time').dataset.end),
  }));
  const checkedIdxs = new Set(checks.map(cb => parseInt(cb.dataset.idx)));
  const cuts = allSegs
    .filter(s => checkedIdxs.has(s.index))
    .map(s => ({ start_seconds: s.start, end_seconds: s.end }));
  try {
    const res = await fetch(`/api/sponsorblock/${encodeURIComponent(videoId)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cuts }),
    });
    if (res.ok) closeSB(videoId);
  } catch {}
});

// Set name rename with debounce — uses data-old-name to identify which set to rename
const _renameTimers = new WeakMap();
function debounceRename(input) {
  clearTimeout(_renameTimers.get(input));
  _renameTimers.set(input, setTimeout(() => {
    const oldName = input.dataset.oldName;
    const name = input.value;
    fetch('/api/schedule/rename', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, old_name: oldName }),
    }).then(r => { if (r.ok) input.dataset.oldName = name; }).catch(() => {});
  }, 800));
}

// SortableJS drag-and-drop (loaded from CDN)
document.addEventListener('DOMContentLoaded', () => {
  if (typeof Sortable === 'undefined') return;

  Sortable.create(document.getElementById('video-list'), {
    handle: '.set-item > .set-header > .handle',
    animation: 150,
    onEnd: saveOrder,
  });

  document.querySelectorAll('.set-videos').forEach(ul => {
    Sortable.create(ul, {
      handle: '.card > .handle',
      animation: 150,
      onEnd: saveOrder,
    });
  });
});

function saveOrder() {
  const items = [...document.querySelectorAll('#video-list > .set-item')].map(li => ({
    name: li.querySelector('.set-name-input').value,
    video_ids: [...li.querySelectorAll('.card')].map(c => c.dataset.id),
  }));
  fetch('/api/schedule/reorder', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  }).catch(() => {});
}
