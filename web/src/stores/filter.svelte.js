// Shared filter state across views — synced with URL query params
// Persists across reloads via ?category=X&status=Y

function readParam(name) {
  const params = new URLSearchParams(window.location.search);
  return params.get(name) || '';
}

function writeParams() {
  const url = new URL(window.location);
  if (category) url.searchParams.set('category', category);
  else url.searchParams.delete('category');
  if (status) url.searchParams.set('status', status);
  else url.searchParams.delete('status');
  history.replaceState({}, '', url);
}

let category = $state(readParam('category'));
let status = $state(readParam('status'));
let open = $state(false);

export function getFilter() {
  return {
    get category() { return category; },
    set category(val) { category = val; writeParams(); },
    get status() { return status; },
    set status(val) { status = val; writeParams(); },
    get open() { return open; },
    set open(val) { open = val; },
    toggle() { open = !open; },
    sync() {
      category = readParam('category');
      status = readParam('status');
    },
    reset() {
      category = '';
      status = '';
      writeParams();
    },
  };
}

// Listen for browser back/forward to restore filter from URL
window.addEventListener('popstate', () => {
  category = readParam('category');
  status = readParam('status');
});
