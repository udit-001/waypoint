// Shared filter state across views — synced with URL query params.
//
// Five dimensions (WP-96):
//   - categories    multi-value   ?category=Tech,Finance
//   - statuses      multi-value   ?status=Applied,Offer
//   - deadlineBucket single       ?deadline=overdue|this-week|this-month|no-date
//   - stale         toggle        ?stale=1
//   - textQuery     string        ?q=acme
//
// "Filter" narrows the current view. The command palette (WP-94) is a
// different surface with a different job (jump-to-known-thing). Same
// input shape, distinct destinations — do not conflate them.

const DEADLINE_BUCKETS = ['overdue', 'this-week', 'this-month', 'no-date'];
const STALE_THRESHOLD_DAYS = 14;

function readParam(name) {
  const params = new URLSearchParams(window.location.search);
  return params.get(name) || '';
}

function readMultiParam(name) {
  const raw = readParam(name);
  if (!raw) return [];
  return raw.split(',').map(s => s.trim()).filter(Boolean);
}

function readBoolParam(name) {
  return readParam(name) === '1';
}

function writeParams() {
  const url = new URL(window.location);
  // Multi-value params use comma separation.
  if (categories.length) url.searchParams.set('category', categories.join(','));
  else url.searchParams.delete('category');
  if (statuses.length) url.searchParams.set('status', statuses.join(','));
  else url.searchParams.delete('status');
  // Single-value params.
  if (deadlineBucket) url.searchParams.set('deadline', deadlineBucket);
  else url.searchParams.delete('deadline');
  if (stale) url.searchParams.set('stale', '1');
  else url.searchParams.delete('stale');
  if (textQuery) url.searchParams.set('q', textQuery);
  else url.searchParams.delete('q');
  history.replaceState({}, '', url);
}

function readFromUrl() {
  categories = readMultiParam('category');
  statuses = readMultiParam('status');
  deadlineBucket = DEADLINE_BUCKETS.includes(readParam('deadline')) ? readParam('deadline') : '';
  stale = readBoolParam('stale');
  textQuery = readParam('q');
}

let categories = $state(readMultiParam('category'));
let statuses = $state(readMultiParam('status'));
let deadlineBucket = $state(readParam('deadline'));
if (!DEADLINE_BUCKETS.includes(deadlineBucket)) deadlineBucket = '';
let stale = $state(readBoolParam('stale'));
let textQuery = $state(readParam('q'));
let open = $state(false);

export function getFilter() {
  return {
    // ── State ──────────────────────────────────────────
    get categories() { return categories; },
    get statuses() { return statuses; },
    get deadlineBucket() { return deadlineBucket; },
    get stale() { return stale; },
    get textQuery() { return textQuery; },
    get open() { return open; },
    set open(val) { open = val; },

    // ── Derived counts ─────────────────────────────────
    /** Total selected filter values across all dimensions — drives the
     *  TopBar Filter button badge. Each status/category counts as 1;
     *  deadline bucket, stale, and text-query each count as 1 if set. */
    get activeCount() {
      let n = categories.length + statuses.length;
      if (deadlineBucket) n += 1;
      if (stale) n += 1;
      if (textQuery.trim()) n += 1;
      return n;
    },
    /** True when any dimension is set — convenience for hasActiveFilter. */
    get any() { return this.activeCount > 0; },

    // ── Multi-value toggles ────────────────────────────
    toggleStatus(s) {
      const i = statuses.indexOf(s);
      if (i === -1) statuses = [...statuses, s];
      else statuses = statuses.filter(x => x !== s);
      writeParams();
    },
    toggleCategory(c) {
      const i = categories.indexOf(c);
      if (i === -1) categories = [...categories, c];
      else categories = categories.filter(x => x !== c);
      writeParams();
    },
    setStatuses(arr) { statuses = arr || []; writeParams(); },
    setCategories(arr) { categories = arr || []; writeParams(); },

    // ── Single-value setters ────────────────────────────
    setDeadlineBucket(b) {
      deadlineBucket = (b === deadlineBucket) ? '' : b;
      writeParams();
    },
    toggleStale() {
      stale = !stale;
      writeParams();
    },
    setTextQuery(q) {
      textQuery = q || '';
      writeParams();
    },

    // ── Lifecycle ───────────────────────────────────────
    toggle() { open = !open; },
    clear() {
      categories = [];
      statuses = [];
      deadlineBucket = '';
      stale = false;
      textQuery = '';
      writeParams();
    },
    sync() { readFromUrl(); },
    reset() { this.clear(); },
  };
}

// Listen for browser back/forward to restore filter from URL.
window.addEventListener('popstate', readFromUrl);

export { STALE_THRESHOLD_DAYS, DEADLINE_BUCKETS };
