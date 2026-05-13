import Sortable from 'sortablejs';

function fmt(secs) {
  const s = Math.round(secs);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// SortableJS drag-and-drop
function initSortable() {
  document.querySelectorAll('.series-seasons').forEach(ul => {
    Sortable.create(ul, {
      handle: '.set-item > .set-header > .handle',
      animation: 150,
      onEnd: saveOrder,
    });
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
  if (e.target.id === 'series-list') initSortable();
});

function saveOrder() {
  document.querySelectorAll('#series-list > .series-item').forEach(seriesEl => {
    const seriesName = seriesEl.dataset.seriesName;
    const seasons = [...seriesEl.querySelectorAll(':scope > .series-seasons > .set-item')].map(li => ({
      name: li.querySelector('.set-name-input').value,
      episode_ids: [...li.querySelectorAll('.card')].map(c => c.dataset.id),
    }));
    fetch('/api/series/reorder', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ series_name: seriesName, seasons }),
    }).catch(() => {});
  });
}

// Position slider visual feedback — delegated, no global needed.
document.addEventListener('input', e => {
  if (!e.target.matches('.pos-range')) return;
  const input = e.target;
  const wrap = input.closest('.pos-slider-wrap');
  const fill = wrap?.querySelector('.pos-fill');
  const max = parseFloat(input.max) || 1;
  if (fill) fill.style.width = (parseFloat(input.value) / max * 100) + '%';
  const display = wrap?.closest('.pos-wrap')?.querySelector('.pos-display');
  if (display) display.textContent = fmt(parseFloat(input.value));
});

// SponsorBlock panel close — delegated, no global needed.
document.addEventListener('click', e => {
  const btn = e.target.closest('.sb-close');
  if (!btn) return;
  btn.closest('.sb-panel')?.replaceChildren();
});
