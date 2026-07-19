// Reactive page state: title, byline, breadcrumbs, document.title.
// Each view calls setPage() on mount to set the right context.
//
// The byline (WP-95) is a compact one-liner the TopBar renders next to
// the page title — Applications uses it to surface "19 total · 42%
// response" so the Dashboard's stat cards collapse into the header
// instead of taking their own row.

let title = $state('Applications');
let byline = $state('');
let breadcrumbs = $state([]);

export function setPage(opts) {
  title = opts.title || 'Applications';
  byline = opts.byline || '';
  breadcrumbs = opts.breadcrumbs || [];
  document.title = title + ' — Waypoint';
}

export function getPage() {
  return {
    get title() { return title; },
    get byline() { return byline; },
    get breadcrumbs() { return breadcrumbs; },
  };
}
