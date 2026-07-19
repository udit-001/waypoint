// Simple hash-free router that works with Svelte 5 runes.
// Uses the History API + popstate for clean URL navigation.
//
// WP-95: /applications replaces /dashboard, /kanban, /table, /search.
// Those old paths "die with no redirect" — bookmarks fall through to
// the default route (applications), so users land somewhere sensible
// rather than on a 404. The router no longer recognises them as
// distinct routes.

const subscribers = new Set();

function parsePath(pathname) {
  const clean = pathname.replace(/^\/+|\/+$/g, '');

  // /job/:id  and  /artifact/:id  keep their dynamic shape.
  const jobMatch = clean.match(/^job\/(\d+)$/);
  if (jobMatch) return { route: 'job', params: { id: jobMatch[1] } };

  const artifactMatch = clean.match(/^artifact\/(\d+)$/);
  if (artifactMatch) return { route: 'artifact', params: { id: artifactMatch[1] } };

  // Named routes — only the living ones. Old view paths
  // (/dashboard, /kanban, /table, /search) intentionally absent.
  const routes = ['applications', 'categories', 'profile', 'skills', 'artifacts', 'settings'];
  const top = clean.split('?')[0];
  if (routes.includes(top)) return { route: top, params: {} };

  // Default — also the catch-all for retired paths.
  return { route: 'applications', params: {} };
}

let current = $state(parsePath(window.location.pathname));

function navigate(path) {
  history.pushState({}, '', path);
  current = parsePath(window.location.pathname);
}

function replace(path) {
  history.replaceState({}, '', path);
  current = parsePath(window.location.pathname);
}

// Listen for browser back/forward.
window.addEventListener('popstate', () => {
  current = parsePath(window.location.pathname);
});

export function getRouter() {
  return {
    get current() { return current; },
    navigate,
    replace,
  };
}
