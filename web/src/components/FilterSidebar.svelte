<script>
  import { onMount } from 'svelte';
  import { getFilter } from '../stores/filter.svelte.js';
  import * as api from '../stores/api.svelte.js';
  import { iconSvg } from '../lib/icons.js';

  const filter = getFilter();

  let cats = $state([]);
  let jobCounts = {};

  onMount(async () => {
    await Promise.all([api.categories.ensure(), api.jobs.ensure()]);
    cats = api.categories.value || [];
    const counts = {};
    (api.jobs.value || []).forEach(j => {
      const c = j.category || 'General';
      counts[c] = (counts[c] || 0) + 1;
    });
    jobCounts = counts;
  });

  function select(cat) {
    filter.category = filter.category === cat ? '' : cat;
  }
</script>

<!-- Overlay when open -->
{#if filter.open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div
    class="fixed inset-0 z-30 bg-black/20 backdrop-blur-sm"
    onclick={filter.toggle}
  ></div>
{/if}

<!-- Filter panel -->
<div
  class="fixed top-0 right-0 z-40 h-full bg-slate-50 dark:bg-slate-800 border-l border-slate-200 dark:border-slate-600 shadow-lg transition-all duration-200 overflow-y-auto {filter.open ? 'w-64' : 'w-0 border-l-0 overflow-hidden'}"
>
  {#if filter.open}
    <div class="p-4">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-200">Filters</h3>
        <button
          class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer bg-transparent border-none p-1"
          onclick={filter.toggle}
        >{@html iconSvg('close', 16)}</button>
      </div>

      {#if filter.category}
        <div class="mb-4">
          <span class="text-xs text-slate-400">Active filter:</span>
          <span class="ml-1.5 inline-flex items-center gap-1 bg-slate-700 text-white text-xs rounded-full px-2.5 py-1">
            {filter.category}
            <button class="text-white/70 hover:text-white cursor-pointer bg-transparent border-none p-0 text-xs" onclick={filter.reset}>×</button>
          </span>
        </div>
      {/if}

      <div class="space-y-1">
        <button
          class="w-full text-left px-3 py-2 rounded-lg text-sm cursor-pointer transition-colors {!filter.category ? 'bg-slate-800 text-white dark:bg-slate-600' : 'text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'}"
          onclick={filter.reset}
        >
          <div class="flex justify-between items-center">
            <span>All Categories</span>
            <span class="text-xs opacity-60">{Object.values(jobCounts).reduce((a, b) => a + b, 0)}</span>
          </div>
        </button>
        {#each cats as cat}
          <button
            class="w-full text-left px-3 py-2 rounded-lg text-sm cursor-pointer transition-colors {filter.category === cat.name ? 'bg-slate-800 text-white dark:bg-slate-600' : 'text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'}"
            onclick={() => select(cat.name)}
          >
            <div class="flex justify-between items-center">
              <span>{cat.name}</span>
              <span class="text-xs opacity-60">{jobCounts[cat.name] || 0}</span>
            </div>
          </button>
        {/each}
      </div>
    </div>
  {/if}
</div>
