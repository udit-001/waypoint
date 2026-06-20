/** Base fetch helper — all stores go through this. */
async function api(path) {
  const res = await fetch('/api' + path);
  if (!res.ok) throw new Error(`API ${res.status}: ${res.statusText}`);
  return res.json();
}

/**
 * Create a read-only store that fetches from the API.
 * - Auto-fetches on first subscription (lazy init)
 * - .refresh() re-fetches and updates subscribers
 * - .loading and .error are reactive
 */
export function createStore(fetchFn) {
  let value = $state(null);
  let loading = $state(false);
  let error = $state(null);
  let loaded = false;

  async function refresh() {
    loading = true;
    error = null;
    try {
      value = await fetchFn();
    } catch (e) {
      error = e.message || 'Fetch failed';
      // On first load failure, default to empty data
      if (!loaded) value = null;
    } finally {
      loading = false;
      loaded = true;
    }
  }

  return {
    get value() { return value; },
    get loading() { return loading; },
    get error() { return error; },
    refresh,
    /** Ensure data is loaded (idempotent). */
    async ensure() {
      if (!loaded) await refresh();
    },
  };
}

// ─── Jobs ───────────────────────────────────────────────

export const jobs = createStore(async () => {
  const data = await api('/jobs');
  return Array.isArray(data) ? data : [];
});

export async function getJob(id) {
  return api(`/jobs/${id}`);
}

export async function searchJobs(query, status, category) {
  const params = new URLSearchParams();
  if (query) params.set('search', query);
  if (status) params.set('status', status);
  if (category) params.set('category', category);
  const data = await api('/jobs?' + params.toString());
  return Array.isArray(data) ? data : [];
}

// ─── Stats ──────────────────────────────────────────────

export const stats = createStore(async () => {
  return api('/stats');
});

// ─── Categories ─────────────────────────────────────────

export const categories = createStore(async () => {
  const data = await api('/categories');
  return Array.isArray(data) ? data : [{ id: 1, name: 'General' }];
});

// ─── History ────────────────────────────────────────────

export const history = createStore(async () => {
  const data = await api('/history');
  return Array.isArray(data) ? data : [];
});

export async function getJobHistory(jobId) {
  const data = await api(`/jobs/${jobId}/history`);
  return Array.isArray(data) ? data : [];
}

// ─── Profile ────────────────────────────────────────────

export const profile = createStore(async () => {
  const p = await api('/profile');
  if (!p) return null;
  // API now returns arrays directly (WAYP-18 shipped)
  return p;
});

// ─── Settings ───────────────────────────────────────────

export const settings = createStore(async () => {
  const s = await api('/settings');
  if (!s) return { theme: 'light', remindersEnabled: true, defaultView: 'dashboard', itemsPerPage: 25 };
  return {
    theme: s.theme || 'light',
    remindersEnabled: Boolean(s.remindersEnabled),
    defaultView: s.defaultView || 'dashboard',
    itemsPerPage: s.itemsPerPage || 25,
  };
});

// ─── Artifacts ──────────────────────────────────────────

export const artifacts = createStore(async () => {
  const data = await api('/artifacts');
  return Array.isArray(data) ? data : [];
});

export async function getArtifact(id) {
  const art = await api(`/artifacts/${id}`);
  return art || null;
}

export async function searchArtifacts(query) {
  const data = await api('/artifacts?search=' + encodeURIComponent(query));
  return Array.isArray(data) ? data : [];
}

// ─── Unified Search ─────────────────────────────────────

export async function searchAll(query) {
  const data = await api('/search?q=' + encodeURIComponent(query));
  return data || [];
}
