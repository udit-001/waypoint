<script>
  // Companies — the boards.toml monitoring surface (WP-149).
  //
  // Read-only by design (controls land in WP-155). Calm at zero, loud on
  // news: a row's attention weight is carried left-to-right — new-postings
  // count first (accent when >0, near-invisible dash at 0 — never bright
  // zeros), company name with its provider chip subordinate, and the
  // trust strip on the right (relative last-swept time, amber when stale,
  // an inline diagnostic when the last sweep failed). Sort comes from the
  // server: companies with new postings float to the top.

  import { onMount } from 'svelte';
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { relTime } from '../lib/format.js';
  import Spinner from '../components/Spinner.svelte';
  import * as api from '../stores/api.svelte.js';

  let companies = $state([]);
  let loaded = $state(false);

  onMount(async () => {
    setPage({ title: 'Companies' });
    try {
      await api.companies.ensure();
      companies = api.companies.value || [];
    } finally {
      loaded = true;
    }
  });
</script>

{#if !loaded && companies.length === 0}
  <Spinner text="Loading companies..." />
{:else}
  <div class="space-y-4">
    <p class="text-sm text-slate-400 mb-4">
      Watched companies and their board health. Add one with <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint boards add</code>.
    </p>

    {#if companies.length === 0}
      <div class="text-center py-12">
        <svg class="mx-auto text-slate-300 mb-3" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/><path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/><path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2"/></svg>
        <p class="text-sm text-slate-400 mb-1">No companies yet</p>
        <p class="text-xs text-slate-400">Add one with <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint boards add</code></p>
      </div>
    {:else}
      <ul class="divide-y divide-slate-100 dark:divide-slate-700 border-y border-slate-100 dark:border-slate-700">
        {#each companies as c (c.name)}
          <li class="flex items-center gap-3 py-2.5 px-1 hover:bg-slate-50 dark:hover:bg-slate-700/40 transition-colors">
            <!-- New postings — accent only when there is news -->
            <span
              class="w-9 text-right tabular-nums font-semibold shrink-0 {c.newCount > 0
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-slate-300/60 dark:text-slate-600'}"
              title={c.newCount > 0 ? `${c.newCount} new posting(s) awaiting review` : 'No new postings'}
            >
              {c.newCount > 0 ? c.newCount : '—'}
            </span>

            <!-- Identity: name leads, provider chip subordinate -->
            <div class="min-w-0 flex items-baseline gap-2">
              <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-200">{c.company || c.name}</span>
              {#if c.provider}
                <span class="shrink-0 px-1.5 py-px rounded bg-slate-100 dark:bg-slate-700 text-[10px] font-mono uppercase tracking-wide text-slate-500 dark:text-slate-400">{c.provider}</span>
              {/if}
              {#if !c.enabled}
                <span class="text-[10px] uppercase tracking-wide text-slate-400">paused</span>
              {/if}
            </div>

            <!-- Trust strip -->
            <div class="ml-auto flex items-center gap-2 shrink-0 text-xs">
              {#if c.lastSweepError}
                <span class="text-amber-600 dark:text-amber-400" title={c.lastSweepError}>⚠ {c.lastSweepError}</span>
              {:else if c.lastSweptAt}
                <span class="{c.stale ? 'text-amber-600 dark:text-amber-400' : 'text-slate-400 dark:text-slate-500'}"
                  title={'Last swept ' + relTime(c.lastSweptAt)}>
                  swept {relTime(c.lastSweptAt)}
                </span>
              {:else}
                <span class="text-slate-300 dark:text-slate-600">never swept</span>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
