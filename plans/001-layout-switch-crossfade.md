# Plan 001 — List/Kanban layout switch crossfade

**Commit:** eac6731 lineage · **Severity:** HIGH · **Effort:** ~20min

## Context
Applications.svelte renders list or kanban via `{#if layoutStore.current === 'kanban'}` branch chain (lines ~216/318). Toggling is a hard cut: no transition on either branch root, and the per-row `stagger()` helper zeroes durations after first render.

## Current code (branch roots)
Kanban root (~line 318):
```svelte
{:else if layoutStore.current === 'kanban'}
  <!-- Velocity chart ... -->
  <div class="-mx-6 px-6 -mb-6 flex gap-3 h-[calc(100vh-3.5rem)] pb-4 pt-6 overflow-x-auto overflow-y-hidden">
```
List root (~line 397):
```svelte
{:else}
  <!-- Velocity chart ... -->
  <div class="-mx-6">
```

## Change
1. Add to imports (line 19): `import { fly } from 'svelte/transition';` exists — add `import { expoOut } from 'svelte/easing';`
2. Kanban inner scroll container gets: `in:fly={{ y: 6, duration: 170, easing: expoOut }}`
   (put it on the flex container div, NOT the chart wrapper)
3. List container `<div class="-mx-6">` gets: `in:fly={{ y: 6, duration: 170, easing: expoOut }}`
4. NO out-transitions (simultaneous in+out causes double-flow layout jump). Incoming rises, outgoing vanishes — reads as swap, not slide.

## Conventions honored
- 170ms ≈ `--dur-base` (200ms) tightened for a frequent toggle; expoOut matches `--ease-out` curve family
- `prefers-reduced-motion`: app.css global kill-switch handles it — no extra work
- Row stagger stays untouched (by design)

## Scope boundaries
- Do NOT touch TopBar pills (plan 003)
- Do NOT add out: transitions
- Do NOT change stagger()

## Verification
1. Toggle List↔Kanban repeatedly fast — no stacking, no scroll jump, content rises into place
2. DevTools Rendering → emulate reduced motion — switch is instant, still correct
3. First page load unchanged (skeletons → rows)
