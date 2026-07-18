// WP-95 — reactive layout store for the Applications view.
//
// The pure helpers in lib/layout.js own the URL + localStorage math;
// this store wraps them in Svelte 5 runes so components can $derive
// off the current layout and a single setter (used by TopBar's
// segmented List/Kanban toggle) writes through to URL + storage +
// every subscriber in one go.
//
// Pattern mirrors commandPalette.svelte.js — module-level $state, a
// getLayout()/setLayout() pair, popstate listener to follow the
// browser's back/forward so manual URL edits converge.

import { resolveLayout, saveLayout } from '../lib/layout.js';

let layout = $state(resolveLayout());

if (typeof window !== 'undefined') {
  window.addEventListener('popstate', () => {
    layout = resolveLayout();
  });
}

export function getLayout() {
  return {
    get current() { return layout; },
    set(next) {
      // saveLayout normalises + writes URL + localStorage. We mirror
      // the normalised result into the reactive slot so a bogus
      // value (e.g. setLayout('garbage')) still settles on 'list'.
      layout = saveLayout(next);
    },
  };
}
