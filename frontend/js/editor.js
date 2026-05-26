import Sortable from 'sortablejs';

function fmt(secs) {
  const s = Math.round(secs);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// SortableJS drag-and-drop scoped to the visible series detail panel.
function initSortable() {
  const detail = document.getElementById('series-detail');
  if (!detail) return;
  detail.querySelectorAll('.series-seasons').forEach(ul => {
    Sortable.create(ul, {
      handle: '.set-item > .set-header > .handle',
      animation: 150,
      onEnd: saveOrder,
    });
  });
  detail.querySelectorAll('.set-videos').forEach(ul => {
    Sortable.create(ul, {
      handle: '.card > .handle',
      animation: 150,
      onEnd: saveOrder,
    });
  });
}

function updateNavSelection() {
  const detailId = document.getElementById('series-detail')?.dataset.seriesId;
  document.querySelectorAll('.series-nav-item').forEach(el =>
    el.classList.toggle('selected', el.dataset.seriesId === detailId));
}

function scrollToActive() {
  const active = document.querySelector('#series-detail-wrap .card.active');
  if (active) active.scrollIntoView({ block: 'nearest', behavior: 'instant' });
}

let savedDetailScroll = 0;

document.addEventListener('htmx:beforeSwap', e => {
  if (e.detail.target?.id === 'series-list') {
    savedDetailScroll = document.getElementById('series-detail-wrap')?.scrollTop ?? 0;
  }
});

document.addEventListener('DOMContentLoaded', () => { initSortable(); scrollToActive(); });
document.addEventListener('htmx:afterSwap', e => {
  const targetId = e.detail.target?.id;
  if (targetId === 'series-list' || targetId === 'series-detail-wrap' || targetId === 'series-detail') {
    initSortable();
  }
  if (targetId === 'series-list') {
    const wrap = document.getElementById('series-detail-wrap');
    if (wrap) wrap.scrollTop = savedDetailScroll;
  } else if (targetId === 'series-detail-wrap' || targetId === 'series-detail') {
    scrollToActive();
  }
  updateNavSelection();
});

function saveOrder() {
  const detail = document.getElementById('series-detail');
  if (!detail) return;
  const seriesName = detail.dataset.seriesId;
  const seasons = [...detail.querySelectorAll(':scope > .series-seasons > .set-item')].map(li => ({
    name: li.querySelector('.set-name-input').value,
    episode_ids: [...li.querySelectorAll('.card')].map(c => c.dataset.id),
  }));
  fetch('/api/series/reorder', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ series_name: seriesName, seasons }),
  }).catch(() => {});
}

// Episode filter — delegated, no global needed.
document.addEventListener('input', e => {
  const input = e.target.closest('.filter-input');
  if (!input) return;
  const q = input.value.trim().toLowerCase();
  const detail = document.getElementById('series-detail');
  if (!detail) return;
  detail.querySelectorAll('.set-item').forEach(season => {
    if (!q) {
      season.style.display = '';
      season.querySelectorAll('.card').forEach(c => c.style.display = '');
      return;
    }
    const seasonName = season.querySelector('.set-name-input')?.value.toLowerCase() ?? '';
    const seasonMatches = seasonName.includes(q);
    let anyEpisodeVisible = false;
    season.querySelectorAll('.card').forEach(card => {
      const title = card.querySelector('.title')?.textContent.toLowerCase() ?? '';
      const show = seasonMatches || title.includes(q);
      card.style.display = show ? '' : 'none';
      if (show) anyEpisodeVisible = true;
    });
    season.style.display = anyEpisodeVisible ? '' : 'none';
  });
});

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
