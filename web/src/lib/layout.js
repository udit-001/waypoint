// WP-95 — layout persistence for the unified Applications view.
//
// Two layouts ("list" | "kanban") live on one route (/applications).
// The active layout is:
//   1. Read from the URL (?layout=kanban) — shareable, survives reload
//   2. Falling back to localStorage (`waypoint_applications_layout`)
//   3. Falling back to the default ("list")
//
// Writes go to BOTH the URL (via replaceState, so it doesn't push
// history entries) and localStorage. This mirrors lific's saveLayout
// pattern (../lific/web/src/lib/issues/views.ts), minus the per-project
// scoping — Waypoint is single-pool.

export const LAYOUTS = ['list', 'kanban'];
export const DEFAULT_LAYOUT = 'list';
const STORAGE_KEY = 'waypoint_applications_layout';

/** Coerce an arbitrary value into a valid layout, falling back to
 *  DEFAULT_LAYOUT. Returns the layout if valid, otherwise the default.
 *  Exported so tests + the URL param check share one source of truth. */
export function normalizeLayout(value) {
  return LAYOUTS.includes(value) ? value : DEFAULT_LAYOUT;
}

/** Read the layout from a URLSearchParams. Used by the router on
 *  navigation and by Applications.svelte on mount. */
export function layoutFromParams(params) {
  return normalizeLayout(params && params.get('layout'));
}

/** Browser-side read: pull layout from the current URL's query string. */
export function layoutFromUrl() {
  if (typeof window === 'undefined') return DEFAULT_LAYOUT;
  return layoutFromParams(new URLSearchParams(window.location.search));
}

/** Read the cached layout from localStorage. Returns DEFAULT_LAYOUT if
 *  storage is unavailable (private mode) or the stored value is invalid.
 *  The URL wins when both are set — the URL is the shareable source. */
export function layoutFromStorage() {
  if (typeof localStorage === 'undefined') return DEFAULT_LAYOUT;
  try {
    return normalizeLayout(localStorage.getItem(STORAGE_KEY));
  } catch {
    return DEFAULT_LAYOUT;
  }
}

/** Resolve the effective layout: URL first, then localStorage, then
 *  default. The URL is authoritative when present (shareable links win
 *  over the user's last local choice). */
export function resolveLayout() {
  if (typeof window === 'undefined') return DEFAULT_LAYOUT;
  const params = new URLSearchParams(window.location.search);
  // has('layout') distinguishes "URL had no ?layout=" from
  // "?layout=garbage" — both normalize to DEFAULT_LAYOUT, but only the
  // former should fall through to localStorage.
  if (params.has('layout')) return normalizeLayout(params.get('layout'));
  return layoutFromStorage();
}

/** Write the layout to both the URL (replaceState — no history noise)
 *  and localStorage. Idempotent. */
export function saveLayout(layout) {
  const next = normalizeLayout(layout);
  if (typeof window !== 'undefined') {
    const url = new URL(window.location);
    if (next === DEFAULT_LAYOUT) url.searchParams.delete('layout');
    else url.searchParams.set('layout', next);
    history.replaceState({}, '', url);
  }
  if (typeof localStorage !== 'undefined') {
    try { localStorage.setItem(STORAGE_KEY, next); } catch { /* private mode */ }
  }
  return next;
}
