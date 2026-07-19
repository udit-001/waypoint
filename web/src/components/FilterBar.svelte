<script>
  // WP-96 — chip strip for the active filter dimensions.
  // Renders one chip per active filter value across all 5 dimensions:
  //   - each selected status (e.g. "Applied ×", "Offer ×")
  //   - each selected category (e.g. "Tech ×", "Uncategorized ×")
  //   - deadline bucket (e.g. "Deadline: This week ×")
  //   - stale toggle ("Stale ×")
  //   - text-query ("Text: "acme" ×")
  // Hides entirely when no filters are active.
  //
  // "Clear all" on the right; result count on the right too.

  import { fade } from 'svelte/transition';
  import { getFilter } from '../stores/filter.svelte.js';
  import * as api from '../stores/api.svelte.js';
  import { applyFilter } from '../lib/filter.js';
  import { STATUS_META } from '../lib/status.js';
  import { iconSvg } from '../lib/icons.js';

  const filter = getFilter();

  let totalJobs = $derived((api.jobs.value || []).length);
  let filteredJobs = $derived(applyFilter(api.jobs.value, filter));

  const DEADLINE_LABELS = {
    'overdue': 'Overdue',
    'this-week': 'This week',
    'this-month': 'This month',
    'no-date': 'No date',
  };

  // Build a flat list of chips from the filter state. Each chip knows how
  // to remove itself (the `remove` callback).
  let chips = $derived.by(() => {
    const list = [];
    for (const s of filter.statuses) {
      const meta = STATUS_META[s];
      list.push({
        key: 'status-' + s,
        label: s,
        icon: meta?.icon,
        color: meta?.color,
        remove: () => filter.toggleStatus(s),
      });
    }
    for (const c of filter.categories) {
      list.push({ key: 'cat-' + c, label: c || 'Uncategorized', remove: () => filter.toggleCategory(c) });
    }
    if (filter.deadlineBucket) {
      const label = 'Deadline: ' + DEADLINE_LABELS[filter.deadlineBucket];
      list.push({ key: 'deadline', label, remove: () => filter.setDeadlineBucket(filter.deadlineBucket) });
    }
    if (filter.stale) {
      list.push({ key: 'stale', label: 'Stale', remove: () => filter.toggleStale() });
    }
    const q = filter.textQuery.trim();
    if (q) {
      list.push({ key: 'text', label: 'Text: "' + q + '"', remove: () => filter.setTextQuery('') });
    }
    return list;
  });
</script>

{#if chips.length > 0}
  <div transition:fade={{ duration: 200 }} class="flex items-center gap-2 px-6 py-2 bg-stone-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-600 text-xs">
    <span class="text-slate-500 dark:text-slate-400 tabular-nums shrink-0">
      {filteredJobs.length} of {totalJobs} results
    </span>
    <span class="text-slate-300 dark:text-slate-600 shrink-0">|</span>

    {#each chips as chip (chip.key)}
      <span class="inline-flex items-center gap-1 bg-slate-700 text-white rounded-full px-2.5 py-0.5 text-[11px] font-medium">
        {#if chip.icon}
          <span style="color: {chip.color}">{@html iconSvg(chip.icon, 10, { duotone: false })}</span>
        {/if}
        {chip.label}
        <button
          class="text-white/60 hover:text-white cursor-pointer bg-transparent border-none p-0 text-xs leading-none inline-flex items-center justify-center"
          onclick={chip.remove}
          aria-label="Remove filter: {chip.label}"
        >×</button>
      </span>
    {/each}

    <button
      class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer bg-transparent border-none p-0 text-xs underline ml-auto shrink-0"
      onclick={() => filter.clear()}
    >Clear all</button>
  </div>
{/if}
