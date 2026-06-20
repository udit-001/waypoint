// Reactive page state: title, breadcrumbs, document.title
// Each view calls setPage() on mount to set the right context.

let title = $state('Dashboard');
let breadcrumbs = $state([]);

export function setPage(opts) {
  title = opts.title || 'Dashboard';
  breadcrumbs = opts.breadcrumbs || [];
  document.title = title + ' — Waypoint';
}

export function getPage() {
  return {
    get title() { return title; },
    get breadcrumbs() { return breadcrumbs; },
  };
}
