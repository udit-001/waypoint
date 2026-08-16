// Profile-page tab persistence — mirrors lib/layout.js (the Applications
// view's list/kanban toggle).
//
// Two tabs live on one route (/profile): "profile" (import + identity cards)
// and "preferences" (the job-search brief). The active tab is:
//   1. Read from the URL (?tab=preferences) — shareable, survives reload
//   2. Falling back to localStorage (`waypoint_profile_tab`)
//   3. Falling back to the default ("profile")
//
// Writes go to BOTH the URL (via replaceState, so it doesn't push history
// entries) and localStorage — same contract as layout.js.

export const PROFILE_TABS = ['profile', 'preferences'];
export const DEFAULT_PROFILE_TAB = 'profile';
const STORAGE_KEY = 'waypoint_profile_tab';

/** Coerce an arbitrary value into a valid tab, falling back to
 *  DEFAULT_PROFILE_TAB. Exported so tests + the URL param check share one
 *  source of truth. */
export function normalizeTab(value) {
  return PROFILE_TABS.includes(value) ? value : DEFAULT_PROFILE_TAB;
}

/** Read the tab from a URLSearchParams. Used by the store on navigation. */
export function tabFromParams(params) {
  return normalizeTab(params && params.get('tab'));
}

/** Browser-side read: pull the tab from the current URL's query string. */
export function tabFromUrl() {
  if (typeof window === 'undefined') return DEFAULT_PROFILE_TAB;
  return tabFromParams(new URLSearchParams(window.location.search));
}

/** Read the cached tab from localStorage. Returns DEFAULT_PROFILE_TAB if
 *  storage is unavailable (private mode) or the stored value is invalid.
 *  The URL wins when both are set — the URL is the shareable source. */
export function tabFromStorage() {
  if (typeof localStorage === 'undefined') return DEFAULT_PROFILE_TAB;
  try {
    return normalizeTab(localStorage.getItem(STORAGE_KEY));
  } catch {
    return DEFAULT_PROFILE_TAB;
  }
}

/** Resolve the effective tab: URL first, then localStorage, then default.
 *  The URL is authoritative when present (shareable links win over the
 *  user's last local choice). */
export function resolveTab() {
  if (typeof window === 'undefined') return DEFAULT_PROFILE_TAB;
  const params = new URLSearchParams(window.location.search);
  // has('tab') distinguishes "URL had no ?tab=" from "?tab=garbage" — both
  // normalize to DEFAULT_PROFILE_TAB, but only the former should fall
  // through to localStorage.
  if (params.has('tab')) return normalizeTab(params.get('tab'));
  return tabFromStorage();
}

/** Write the tab to both the URL (replaceState — no history noise) and
 *  localStorage. Idempotent. */
export function saveTab(tab) {
  const next = normalizeTab(tab);
  if (typeof window !== 'undefined') {
    const url = new URL(window.location);
    if (next === DEFAULT_PROFILE_TAB) url.searchParams.delete('tab');
    else url.searchParams.set('tab', next);
    history.replaceState({}, '', url);
  }
  if (typeof localStorage !== 'undefined') {
    try { localStorage.setItem(STORAGE_KEY, next); } catch { /* private mode */ }
  }
  return next;
}
