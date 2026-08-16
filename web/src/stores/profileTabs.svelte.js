// Reactive tab state for the Profile page — mirrors layout.svelte.js (the
// Applications view's List/Kanban toggle).
//
// The pure helpers in lib/profileTabs.js own the URL + localStorage math;
// this store wraps them in Svelte 5 runes so components can $derive off the
// current tab and a single setter (the tab bar) writes through to URL +
// storage + every subscriber in one go. A popstate listener follows the
// browser's back/forward so manual URL edits converge.

import { resolveTab, saveTab } from '../lib/profileTabs.js';

let tab = $state(resolveTab());

if (typeof window !== 'undefined') {
  window.addEventListener('popstate', () => {
    tab = resolveTab();
  });
}

export function getProfileTabs() {
  return {
    get current() { return tab; },
    set(next) {
      // saveTab normalises + writes URL + localStorage. We mirror the
      // normalised result into the reactive slot so a bogus value still
      // settles on a valid tab.
      tab = saveTab(next);
    },
  };
}
