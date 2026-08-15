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
// Profile view mode (WP-117): false = clean read-only render, true = forms.
// Transient page state — the profile view resets it on mount (first-run
// auto-enters edit), so edit mode never sticks across navigation.
let editing = $state(false);

export function setPage(opts) {
  title = opts.title || 'Applications';
  byline = opts.byline || '';
  breadcrumbs = opts.breadcrumbs || [];
  if (opts.editing !== undefined) editing = opts.editing;
  document.title = title + ' — Waypoint';
}

export function setEditing(v) {
  editing = v;
}

export function getPage() {
  return {
    get title() { return title; },
    get byline() { return byline; },
    get breadcrumbs() { return breadcrumbs; },
    get editing() { return editing; },
  };
}
