// Simple hash-free router that works with Svelte 5 runes.
// Uses the History API + popstate for clean URL navigation.

const subscribers = new Set();

let current = $state(parsePath(window.location.pathname));

function parsePath(pathname) {
  const path = pathname.replace(/^\/+|\/+$/g, '').split('?')[0];

  // /job/:id
  const jobMatch = path.match(/^job\/(\d+)$/);
  if (jobMatch) return { route: 'job', params: { id: jobMatch[1] } };

  // /artifact/:id
  const artifactMatch = path.match(/^artifact\/(\d+)$/);
  if (artifactMatch) return { route: 'artifact', params: { id: artifactMatch[1] } };

  // Named routes
  const routes = ['dashboard', 'kanban', 'table', 'categories', 'profile', 'skills', 'artifacts', 'settings', 'search'];
  if (routes.includes(path)) return { route: path, params: {} };

  // Default
  return { route: 'dashboard', params: {} };
}

function navigate(path) {
  history.pushState({}, '', path);
  current = parsePath(path);
}

function replace(path) {
  history.replaceState({}, '', path);
  current = parsePath(path);
}

// Listen for browser back/forward
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
