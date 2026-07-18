<script>
  // WP-97 — derived-predicate filter presets + chart toggle.
  // Two presets only (Stale, Upcoming) — the byline + group headers
  // carry Total/Active/Offers counts, so the strip carries only the
  // predicates that don't have another home (Tufte principle #4: no
  // redundant counts).
  //
  // Preset = filter, not section. Clicking narrows the list to only
  // matching jobs; no dimmed "rest" below, no divider. Both presets
  // toggle: click again to clear.

  import { isStale, bucketFor } from '../lib/filter.js';

  let { jobs = [], filter, chartsOpen = $bindable(false) } = $props();

  let staleCount = $derived((jobs || []).filter(isStale).length);
  let upcomingCount = $derived((jobs || []).filter(j => bucketFor(j) === 'this-week').length);

  let staleActive = $derived(filter.stale === true);
  let upcomingActive = $derived(filter.deadlineBucket === 'this-week');

  function applyStale() { filter.toggleStale(); }
  function applyUpcoming() { filter.setDeadlineBucket('this-week'); }
  function toggleCharts() { chartsOpen = !chartsOpen; }
</script>

<div class="flex items-center gap-2 px-6 py-2 bg-slate-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700">
  <span class="text-[10px] uppercase tracking-wider text-slate-400 dark:text-slate-500 font-semibold mr-1">Attention</span>

  <button
    class="flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-xs font-medium transition-colors cursor-pointer
      {staleActive
        ? 'border-amber-400 bg-amber-50 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300'
        : 'border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-600 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-500'}"
    onclick={applyStale}
    aria-pressed={staleActive}
  >
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="10"/>
      <line x1="12" y1="8" x2="12" y2="12"/>
      <line x1="12" y1="16" x2="12.01" y2="16"/>
    </svg>
    <span>Stale</span>
    <span class="font-semibold tabular-nums {staleCount === 0 ? 'opacity-40' : ''}">{staleCount}</span>
  </button>

  <button
    class="flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-xs font-medium transition-colors cursor-pointer
      {upcomingActive
        ? 'border-blue-400 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
        : 'border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-600 dark:text-slate-300 hover:border-slate-300 dark:hover:border-slate-500'}"
    onclick={applyUpcoming}
    aria-pressed={upcomingActive}
  >
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="12" cy="12" r="10"/>
      <polyline points="12 6 12 12 16 14"/>
    </svg>
    <span>Upcoming</span>
    <span class="font-semibold tabular-nums {upcomingCount === 0 ? 'opacity-40' : ''}">{upcomingCount}</span>
  </button>

  <div class="ml-auto flex items-center gap-3 text-xs">
    <button
      class="text-blue-600 dark:text-blue-400 hover:underline font-medium flex items-center gap-1 cursor-pointer bg-transparent border-none p-0"
      onclick={toggleCharts}
      aria-expanded={chartsOpen}
    >
      {chartsOpen ? 'Hide charts' : 'Show charts'}
      <svg class="transition-transform {chartsOpen ? 'rotate-180' : ''}" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </button>
  </div>
</div>
