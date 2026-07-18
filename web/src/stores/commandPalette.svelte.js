// Shared open/close state for the command palette. The palette component
// (mounted once in App.svelte) reads/writes `open`; any other component
// (e.g. the TopBar trigger button) writes `open = true` to summon it.
// Keeping this in its own module lets the trigger live anywhere without
// prop-drilling or bind:this across the tree.

let open = $state(false);

export function getCommandPalette() {
  return {
    get open() { return open; },
    set open(v) { open = v; },
    summon() { open = true; },
    dismiss() { open = false; },
  };
}
