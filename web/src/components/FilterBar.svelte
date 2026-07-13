<script>
  import { getFilter } from '../stores/filter.svelte.js';
  import * as api from '../stores/api.svelte.js';
  import { applyFilter } from '../lib/filter.js';

  const filter = getFilter();

  let totalJobs = $derived((api.jobs.value || []).length);
  let filteredJobs = $derived(applyFilter(api.jobs.value, filter));

  function hasActiveFilter() {
    return filter.category || filter.status;
  }
</script>

{#if hasActiveFilter()}
  <div class="flex items-center gap-2 px-6 py-2 bg-stone-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-600 text-xs">
    <span class="text-slate-500 dark:text-slate-400 tabular-nums">
      {filteredJobs.length} of {totalJobs} results
    </span>
    <span class="text-slate-300 dark:text-slate-600">|</span>

    {#each [{ key: 'category', label: filter.category }, { key: 'status', label: filter.status }] as chip}
      {#if chip.label}
        <span class="inline-flex items-center gap-1 bg-slate-700 text-white rounded-full px-2.5 py-0.5 text-[11px] font-medium">
          {chip.label}
          <button
            class="text-white/60 hover:text-white cursor-pointer bg-transparent border-none p-0 text-xs leading-none"
            onclick={() => { filter[chip.key] = ''; }}
          >×</button>
        </span>
      {/if}
    {/each}

    <button
      class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer bg-transparent border-none p-0 text-xs underline ml-auto"
      onclick={filter.reset}
    >Clear all</button>
  </div>
{/if}
