// WP-98 — reactive store for the velocity chart visibility toggle.
//
// Toggle lives in the TopBar (next to List/Kanban); Applications.svelte
// reads `open` to mount VelocityChart. URL-persisted (?charts=1) so the
// preference survives reloads and back/forward. Pattern mirrors
// layout.svelte.js — module-level $state, getter/setter pair, popstate
// listener for browser navigation.

const CHARTS_PARAM = 'charts';

function readFromUrl() {
  if (typeof window === 'undefined') return false;
  const params = new URLSearchParams(window.location.search);
  return params.get(CHARTS_PARAM) === '1';
}

function writeToUrl(open) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location);
  if (open) url.searchParams.set(CHARTS_PARAM, '1');
  else url.searchParams.delete(CHARTS_PARAM);
  history.replaceState({}, '', url);
}

let open = $state(readFromUrl());

if (typeof window !== 'undefined') {
  window.addEventListener('popstate', () => {
    open = readFromUrl();
  });
}

export function getChartsOpen() {
  return {
    get open() { return open; },
    toggle() {
      open = !open;
      writeToUrl(open);
    },
  };
}
