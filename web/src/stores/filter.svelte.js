// Shared filter state across views — synced with URL query params
// Persists across reloads via ?category=X in the URL

function readCategory() {
  const params = new URLSearchParams(window.location.search);
  return params.get('category') || '';
}

function writeCategory(cat) {
  const url = new URL(window.location);
  if (cat) {
    url.searchParams.set('category', cat);
  } else {
    url.searchParams.delete('category');
  }
  history.replaceState({}, '', url);
}

let category = $state(readCategory());
let open = $state(false);

export function getFilter() {
  return {
    get category() { return category; },
    set category(val) {
      category = val;
      writeCategory(val);
    },
    get open() { return open; },
    set open(val) { open = val; },
    toggle() { open = !open; },
    reset() {
      category = '';
      writeCategory('');
    },
  };
}

// Listen for browser back/forward to restore filter from URL
window.addEventListener('popstate', () => {
  category = readCategory();
});
