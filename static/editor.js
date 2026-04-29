// Position slider visual feedback
function updatePosFill(input) {
  const wrap = input.closest('.pos-slider-wrap');
  const fill = wrap.querySelector('.pos-fill');
  const max = parseFloat(input.max) || 1;
  fill.style.width = (parseFloat(input.value) / max * 100) + '%';
  const display = wrap.closest('.pos-wrap')?.querySelector('.pos-display');
  if (display) display.textContent = fmt(parseFloat(input.value));
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

// SortableJS drag-and-drop (loaded from CDN)
function initSortable() {
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
}

document.addEventListener('DOMContentLoaded', initSortable);
document.addEventListener('htmx:afterSwap', e => {
  if (e.target.id === 'video-list') initSortable();
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
