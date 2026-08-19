// Reactive page state: title, byline, status, breadcrumbs, document.title.
// Each view calls setPage() on mount to set the right context.
//
// The byline (WP-95) is a compact one-liner the TopBar renders next to
// the page title — Applications uses it to surface "19 total · 42%
// response" so the Dashboard's stat cards collapse into the header
// instead of taking their own row.
//
// The status chip (WP-141) renders before the byline as a small pill
// with a colored dot. It carries the page's live state — the thing
// that changes on its own (autopilot running, scoring disabled). Tones:
//   live — something is happening right now (pulse dot)
//   warn — degraded, user action fixes it (links to the fix)
//   off  — inactive (links to where it's enabled)
// Byline facts (counts, last-run results) are quieter than the chip by
// design: identity → live state → context facts is the hierarchy.

let title = $state('Applications');
let byline = $state('');
let status = $state(null); // { label, tone: 'live'|'warn'|'off', href? }
let breadcrumbs = $state([]);
// Profile view mode (WP-117): false = clean read-only render, true = forms.
// Transient page state — the profile view resets it on mount (first-run
// auto-enters edit), so edit mode never sticks across navigation.
let editing = $state(false);

export function setPage(opts) {
  title = opts.title || 'Applications';
  byline = opts.byline || '';
  status = opts.status || null;
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
    get status() { return status; },
    get breadcrumbs() { return breadcrumbs; },
    get editing() { return editing; },
  };
}
