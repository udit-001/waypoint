// Shared filter state across views — category filter, open/closed

let category = $state('');
let open = $state(false);

export function getFilter() {
  return {
    get category() { return category; },
    set category(val) { category = val; },
    get open() { return open; },
    set open(val) { open = val; },
    toggle() { open = !open; },
    reset() { category = ''; },
  };
}
