<script>
  import { onMount } from 'svelte';
  import { getFilter } from '../stores/filter.svelte.js';
  import * as api from '../stores/api.svelte.js';
  import { iconSvg } from '../lib/icons.js';

  const filter = getFilter();

  const statuses = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];

  let cats = $state([]);
  let jobCounts = {};
  let statusCounts = {};

  onMount(async () => {
    await Promise.all([api.categories.ensure(), api.jobs.ensure()]);
    cats = api.categories.value || [];
    const catCounts = {};
    const stCounts = {};
    (api.jobs.value || []).forEach(j => {
      const c = j.category || 'General';
      catCounts[c] = (catCounts[c] || 0) + 1;
      stCounts[j.status] = (stCounts[j.status] || 0) + 1;
    });
    jobCounts = catCounts;
    statusCounts = stCounts;
  });

  function selectCategory(cat) {
    filter.category = filter.category === cat ? '' : cat;
  }

  function selectStatus(st) {
    filter.status = filter.status === st ? '' : st;
  }

  function hasActiveFilter() {
    return filter.category || filter.status;
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

      {#if hasActiveFilter()}
        <div class="mb-4 flex flex-wrap gap-1">
          {#if filter.category}
            <span class="inline-flex items-center gap-1 bg-slate-700 text-white text-xs rounded-full px-2.5 py-1">
              {filter.category}
              <button class="text-white/70 hover:text-white cursor-pointer bg-transparent border-none p-0 text-xs" onclick={() => { filter.category = ''; }}>×</button>
            </span>
          {/if}
          {#if filter.status}
            <span class="inline-flex items-center gap-1 bg-slate-700 text-white text-xs rounded-full px-2.5 py-1">
              {filter.status}
              <button class="text-white/70 hover:text-white cursor-pointer bg-transparent border-none p-0 text-xs" onclick={() => { filter.status = ''; }}>×</button>
            </span>
          {/if}
          <button
            class="text-xs text-slate-400 hover:text-slate-600 cursor-pointer bg-transparent border-none p-0 ml-1"
            onclick={filter.reset}
          >Clear all</button>
        </div>
      {/if}

      <!-- Category filter -->
      <div class="mb-5">
        <h4 class="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">Category</h4>
        <div class="space-y-0.5">
          <button
            class="w-full text-left px-3 py-1.5 rounded-lg text-sm cursor-pointer transition-colors {!filter.category && !filter.status ? 'bg-slate-800 text-white dark:bg-slate-600' : 'text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'}"
            onclick={filter.reset}
          >
            <div class="flex justify-between items-center">
              <span>All</span>
              <span class="text-xs opacity-60">{Object.values(jobCounts).reduce((a, b) => a + b, 0)}</span>
            </div>
          </button>
          {#each cats as cat}
            <button
              class="w-full text-left px-3 py-1.5 rounded-lg text-sm cursor-pointer transition-colors {filter.category === cat.name ? 'bg-slate-800 text-white dark:bg-slate-600' : 'text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'}"
              onclick={() => selectCategory(cat.name)}
            >
              <div class="flex justify-between items-center">
                <span>{cat.name}</span>
                <span class="text-xs opacity-60">{jobCounts[cat.name] || 0}</span>
              </div>
            </button>
          {/each}
        </div>
      </div>

      <!-- Status filter -->
      <div>
        <h4 class="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">Status</h4>
        <div class="space-y-0.5">
          {#each statuses as st}
            <button
              class="w-full text-left px-3 py-1.5 rounded-lg text-sm cursor-pointer transition-colors {filter.status === st ? 'bg-slate-800 text-white dark:bg-slate-600' : 'text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'}"
              onclick={() => selectStatus(st)}
            >
              <div class="flex justify-between items-center">
                <span>{st}</span>
                <span class="text-xs opacity-60">{statusCounts[st] || 0}</span>
              </div>
            </button>
          {/each}
        </div>
      </div>
    </div>
  {/if}
</div>
