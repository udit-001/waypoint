<script>
  // WP-96 — Filter modal. Centered overlay (matching the CommandPalette
  // family) with 5 dimensions in a 2-column grid:
  //
  //   Status (multi)    Category (multi)
  //   Deadline (single) Stale (toggle)
  //   Filter by text (col-span-2)
  //
  // "Filter" narrows the current view — distinct from the command palette
  // (WP-94), which jumps to a known thing. Same input shape, different job.
  //
  // Pattern note: the ticket said "popover anchored below the Filter button"
  // but cited lific's FilterModal (a centered modal) as the structural
  // reference. With 5 sections × 16 controls, a popover doesn't fit; a
  // modal does. Surfaced via layers-surface + layers-interaction-flow
  // audits and confirmed with the user.

  import { getFilter, DEADLINE_BUCKETS } from '../stores/filter.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { STATUSES } from '../lib/status.js';
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import * as api from '../stores/api.svelte.js';

  const filter = getFilter();

  // Load categories lazily on mount — the old FilterSidebar used to do
  // this, but it's gone now. Idempotent if already loaded.
  onMount(() => { void api.categories.ensure(); });

  let categories = $derived(api.categories.value || []);

  const DEADLINE_LABELS = {
    'overdue': 'Overdue',
    'this-week': 'This week',
    'this-month': 'This month',
    'no-date': 'No date',
  };

  const DEADLINE_HINTS = {
    'overdue': 'Deadline passed, not yet applied',
    'this-week': 'Next 7 days',
    'this-month': 'Next 8–30 days',
    'no-date': 'No deadline set',
  };

  function close() {
    filter.open = false;
  }

  function onWindowKeydown(e) {
    if (!filter.open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  }
</script>

<svelte:window onkeydown={onWindowKeydown} />

{#if filter.open}
  <!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
  <div
    transition:fade={{ duration: 120 }}
    class="fixed inset-0 z-[90] bg-black/25 flex items-start justify-center pt-[10dvh] px-4"
    onclick={close}
    role="presentation"
  >
    <!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
    <div
      class="w-full max-w-[600px] max-h-[80dvh] flex flex-col bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-xl shadow-[0_16px_48px_rgba(0,0,0,0.28)] overflow-hidden"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-label="Filter applications"
    >
      <!-- Header -->
      <div class="flex items-center gap-3 px-5 py-3 border-b border-slate-200 dark:border-slate-600 shrink-0">
        <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-200">Filters</h2>
        {#if filter.activeCount > 0}
          <span
            class="grid place-items-center min-w-[18px] h-[18px] px-1 rounded-full bg-slate-700 dark:bg-slate-900 text-white font-mono text-[10px] leading-none tabular-nums"
          >{filter.activeCount}</span>
          <button
            class="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer bg-transparent border-none p-0 underline"
            onclick={() => filter.clear()}
          >Clear all</button>
        {/if}
        <button
          class="ml-auto size-7 grid place-items-center rounded-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer bg-transparent border-none transition-colors"
          aria-label="Close filters"
          onclick={close}
        >{@html iconSvg('close', 15)}</button>
      </div>

      <!-- Body: 2-column grid -->
      <div class="overflow-y-auto px-5 py-4 grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-5">

        <!-- STATUS (multi-select) -->
        <section>
          <h4 class="px-1 pb-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">Status</h4>
          <div class="flex flex-col gap-0.5">
            {#each STATUSES as st (st)}
              {@const active = filter.statuses.includes(st)}
              <button
                class="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-left text-sm transition-colors {active ? 'bg-slate-100 dark:bg-slate-700' : 'hover:bg-slate-100 dark:hover:bg-slate-700'} cursor-pointer bg-transparent border-none"
                onclick={() => filter.toggleStatus(st)}
              >
                <span class="size-4 flex items-center justify-center shrink-0 text-slate-700 dark:text-slate-300">
                  {#if active}{@html iconSvg('check', 14)}{/if}
                </span>
                <span class="text-slate-700 dark:text-slate-300">{st}</span>
              </button>
            {/each}
          </div>
        </section>

        <!-- CATEGORY (multi-select, includes Uncategorized pseudo-entry) -->
        <section>
          <h4 class="px-1 pb-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">Category</h4>
          <div class="flex flex-col gap-0.5">
            <!-- Uncategorized pseudo-entry (empty-string value) -->
            <button
              class="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-left text-sm transition-colors {filter.categories.includes('') ? 'bg-slate-100 dark:bg-slate-700' : 'hover:bg-slate-100 dark:hover:bg-slate-700'} cursor-pointer bg-transparent border-none"
              onclick={() => filter.toggleCategory('')}
            >
              <span class="size-4 flex items-center justify-center shrink-0 text-slate-700 dark:text-slate-300">
                {#if filter.categories.includes('')}{@html iconSvg('check', 14)}{/if}
              </span>
              <span class="text-slate-700 dark:text-slate-300 italic">Uncategorized</span>
            </button>
            {#each categories as cat (cat.id)}
              {@const active = filter.categories.includes(cat.name)}
              <button
                class="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-left text-sm transition-colors {active ? 'bg-slate-100 dark:bg-slate-700' : 'hover:bg-slate-100 dark:hover:bg-slate-700'} cursor-pointer bg-transparent border-none"
                onclick={() => filter.toggleCategory(cat.name)}
              >
                <span class="size-4 flex items-center justify-center shrink-0 text-slate-700 dark:text-slate-300">
                  {#if active}{@html iconSvg('check', 14)}{/if}
                </span>
                <span class="text-slate-700 dark:text-slate-300">{cat.name}</span>
              </button>
            {/each}
            {#if categories.length === 0}
              <p class="text-xs text-slate-400 px-2.5 py-1">No categories yet.</p>
            {/if}
          </div>
        </section>

        <!-- DEADLINE (single-select, mutually exclusive) -->
        <section>
          <h4 class="px-1 pb-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">Deadline</h4>
          <div class="flex flex-col gap-0.5">
            {#each DEADLINE_BUCKETS as bucket (bucket)}
              {@const active = filter.deadlineBucket === bucket}
              <button
                class="w-full flex items-start gap-2.5 px-2.5 py-1.5 rounded-md text-left text-sm transition-colors {active ? 'bg-slate-100 dark:bg-slate-700' : 'hover:bg-slate-100 dark:hover:bg-slate-700'} cursor-pointer bg-transparent border-none"
                onclick={() => filter.setDeadlineBucket(bucket)}
              >
                <span class="size-4 flex items-center justify-center shrink-0 mt-0.5 text-slate-700 dark:text-slate-300">
                  {#if active}{@html iconSvg('check', 14)}{/if}
                </span>
                <span class="flex-1 min-w-0">
                  <span class="block text-slate-700 dark:text-slate-300">{DEADLINE_LABELS[bucket]}</span>
                  <span class="block text-xs text-slate-400 dark:text-slate-500">{DEADLINE_HINTS[bucket]}</span>
                </span>
              </button>
            {/each}
          </div>
        </section>

        <!-- STALE (toggle) -->
        <section>
          <h4 class="px-1 pb-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">Stale</h4>
          <button
            class="w-full flex items-start gap-2.5 px-2.5 py-1.5 rounded-md text-left text-sm transition-colors {filter.stale ? 'bg-slate-100 dark:bg-slate-700' : 'hover:bg-slate-100 dark:hover:bg-slate-700'} cursor-pointer bg-transparent border-none"
            onclick={() => filter.toggleStale()}
          >
            <span class="size-4 flex items-center justify-center shrink-0 mt-0.5 text-slate-700 dark:text-slate-300">
              {#if filter.stale}{@html iconSvg('check', 14)}{/if}
            </span>
            <span class="flex-1 min-w-0">
              <span class="block text-slate-700 dark:text-slate-300">Applied &gt; 14 days ago</span>
              <span class="block text-xs text-slate-400 dark:text-slate-500">No response yet</span>
            </span>
          </button>
        </section>

        <!-- FILTER BY TEXT (full-width) -->
        <section class="sm:col-span-2">
          <h4 class="px-1 pb-1.5 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">Filter by text</h4>
          <input
            type="text"
            value={filter.textQuery}
            oninput={(e) => filter.setTextQuery(e.target.value)}
            placeholder="Narrow by company, position, category…"
            class="w-full bg-white dark:bg-slate-700 border border-slate-200 dark:border-slate-600 rounded-md px-2.5 py-1.5 text-sm text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 outline-none focus:border-slate-400 dark:focus:border-slate-500"
          />
          <p class="text-[10px] text-slate-400 dark:text-slate-500 mt-1 px-1">Narrows this view. Different from ⌘K, which jumps.</p>
        </section>
      </div>

      <!-- Footer -->
      <div class="flex items-center gap-3 px-5 py-2.5 border-t border-slate-200 dark:border-slate-600 text-[11px] text-slate-400 dark:text-slate-500 shrink-0">
        <span class="inline-flex items-center gap-1">
          <kbd class="font-mono px-1 py-0.5 rounded border border-slate-200 dark:border-slate-600 bg-slate-100 dark:bg-slate-700">esc</kbd>
          close
        </span>
        <span class="ml-auto">
          {filter.activeCount > 0 ? `${filter.activeCount} active` : 'No filters applied'}
        </span>
      </div>
    </div>
  </div>
{/if}
