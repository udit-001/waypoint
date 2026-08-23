<script>
  // Companies — the boards.toml monitoring surface (WP-149).
  //
  // Read-only by design (controls land in WP-155). Calm at zero, loud on
  // news: a row's attention weight is carried left-to-right — new-postings
  // count first (accent when >0, muted dash at 0 — never bright zeros),
  // company name with its provider chip subordinate, and the trust strip
  // on the right (relative last-swept time, warning when stale, an inline
  // diagnostic when the last sweep failed). Sort comes from the server:
  // companies with new postings float to the top.
  //
  // Color notes: theming swaps CSS variables per [data-theme], so plain
  // (unvarianted) tokens are chosen to pass WCAG AA ≥4.5:1 in BOTH themes:
  // slate-600 secondary text (6.6/5.6), emerald-700 counts (4.8/5.4),
  // amber-700 warnings (6.3/7.2). The zero dash is decorative — the count's
  // sr-only text carries the information.

  import { onMount, onDestroy } from 'svelte';
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { relTime } from '../lib/format.js';
  import Spinner from '../components/Spinner.svelte';
  import * as api from '../stores/api.svelte.js';

  let companies = $state([]);
  let loaded = $state(false);
  let error = $state(null);
  let acting = $state(new Set()); // board names mid-action
  let sweepErr = $state({});      // name -> diagnosed failure
  let confirmRemove = $state(''); // name awaiting removal confirmation
  let undo = $state(null);        // {entry, timer} for remove undo
  let undoTimer = null;

  async function refresh() {
    await api.companies.refresh();
    companies = api.companies.value || [];
  }

  function guard(name) {
    if (acting.has(name)) return false;
    acting.add(name);
    return true;
  }
  function release(name) {
    acting.delete(name);
    acting = new Set(acting);
  }

  onMount(async () => {
    setPage({ title: 'Companies', byline: 'watched by your autopilot' });
    try {
      await api.companies.ensure();
      companies = api.companies.value || [];
    } catch (e) {
      // A failed fetch must not masquerade as "no companies" — say so.
      error = e.message || 'Failed to load companies';
    } finally {
      loaded = true;
    }
  });

  onDestroy(() => { if (undoTimer) clearTimeout(undoTimer); });

  async function setEnabled(c, enable) {
    if (!guard(c.name)) return;
    try {
      await (enable ? api.resumeCompany(c.name) : api.pauseCompany(c.name));
      await refresh();
    } catch (e) {
      sweepErr = { ...sweepErr, [c.name]: e.message };
    } finally {
      release(c.name);
    }
  }

  async function removeCompany(c) {
    if (confirmRemove !== c.name) {
      confirmRemove = c.name; // two-step: the button asks first
      setTimeout(() => { if (confirmRemove === c.name) confirmRemove = ''; }, 4000);
      return;
    }
    confirmRemove = '';
    if (!guard(c.name)) return;
    try {
      await api.removeCompany(c.name);
      const removed = companies.find(x => x.name === c.name);
      companies = companies.filter(x => x.name !== c.name);
      if (undoTimer) clearTimeout(undoTimer);
      undo = { entry: { ...removed }, timer: setTimeout(() => { undo = null; }, 5000) };
    } catch (e) {
      sweepErr = { ...sweepErr, [c.name]: e.message };
    } finally {
      release(c.name);
    }
  }

  async function undoRemove() {
    if (!undo) return;
    clearTimeout(undoTimer);
    const entry = undo.entry;
    undo = null;
    try {
      await api.restoreCompany(entry.name, entry);
      await refresh();
    } catch (e) {
      error = e.message;
    }
  }

  async function sweepNow(c) {
    if (!guard(c.name)) return;
    sweepErr = { ...sweepErr, [c.name]: null };
    try {
      const res = await api.sweepCompany(c.name);
      if (res.meta?.failed) {
        sweepErr = { ...sweepErr, [c.name]: res.meta.error + ' — retry in a minute' };
      } else {
        showToastCount(c.name, res.meta.new);
      }
      await refresh();
    } catch (e) {
      sweepErr = { ...sweepErr, [c.name]: e.message };
    } finally {
      release(c.name);
    }
  }

  let flash = $state(null); // {name, msg} transient per-row success note
  let flashTimer = null;
  function showToastCount(name, n) {
    flash = { name, msg: `+${n} new posting${n === 1 ? '' : 's'}` };
    if (flashTimer) clearTimeout(flashTimer);
    flashTimer = setTimeout(() => { flash = null; }, 4000);
  }
</script>

{#if !loaded && companies.length === 0}
  <Spinner text="Loading companies..." />
{:else}
  <div class="space-y-4">
    <p class="text-sm text-slate-600 mb-4">
      Companies your autopilot watches. Add one with <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint boards add</code>.
    </p>

    {#if error}
      <div class="text-sm text-amber-700">⚠ {error} — the board list lives in boards.toml; check that the server is running.</div>
    {/if}

    {#if !error && companies.length === 0}
      <div class="text-center py-12">
        <svg class="mx-auto text-slate-300 mb-3" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/><path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/><path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2"/></svg>
        <p class="text-sm text-slate-600 mb-1">No companies watched yet</p>
        <p class="text-xs text-slate-600">Add one and your autopilot sweeps its job board every cycle — start with <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint boards add</code></p>
      </div>
    {:else}
      <ul class="divide-y divide-slate-100 dark:divide-slate-700 border-y border-slate-100 dark:border-slate-700">
        {#each companies as c (c.name)}
          <li class="flex items-center gap-3 py-2.5 px-1 hover:bg-white/40 transition-colors {c.enabled ? '' : 'opacity-50'}">
            <!-- New postings — accent only when there is news; the zero
                 dash is decorative (sr-only text carries the state) -->
            <span
              class="w-9 text-right tabular-nums font-semibold shrink-0 {c.newCount > 0
                ? 'text-emerald-700'
                : 'text-slate-400/70'}"
              title={c.newCount > 0 ? `${c.newCount} new posting(s) awaiting review` : 'No new postings'}
            >
              {#if c.newCount > 0}
                {c.newCount}
              {:else}
                <span aria-hidden="true">—</span>
                <span class="sr-only">No new postings</span>
              {/if}
            </span>

            <!-- Identity: name leads, provider chip subordinate -->
            <div class="min-w-0 flex items-baseline gap-2">
              <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-200">{c.company || c.name}</span>
              {#if c.provider}
                <span class="shrink-0 px-1.5 py-px rounded bg-slate-100 dark:bg-slate-700 text-[10px] font-mono uppercase tracking-wide text-slate-600">{c.provider}</span>
              {/if}
              {#if !c.enabled}
                <span class="text-[10px] uppercase tracking-wide text-slate-600">paused</span>
              {/if}
            </div>

            <!-- Trust strip + per-row action feedback -->
            <div class="ml-auto flex items-center gap-2 shrink-0 text-xs">
              {#if sweepErr[c.name]}
                <span class="text-amber-700" title={sweepErr[c.name]}>⚠ {sweepErr[c.name]}</span>
              {:else if flash?.name === c.name}
                <span class="text-emerald-700 font-medium">{flash.msg}</span>
              {/if}
              {#if c.lastSweepError}
                <span class="text-amber-700" title={c.lastSweepError}>⚠ {c.lastSweepError}</span>
              {:else if c.lastSweptAt}
                <span class="{c.stale ? 'text-amber-700' : 'text-slate-600'}"
                  title={'Last swept ' + relTime(c.lastSweptAt)}>
                  swept {relTime(c.lastSweptAt)}
                </span>
              {:else}
                <span class="text-slate-600 italic">never swept</span>
              {/if}
            </div>

            <!-- Controls (WP-155): pause/resume/remove via the boards
                 manager seam; Sweep now runs one board through the shared
                 sweeper and records state like the CLI does. -->
            <div class="flex items-center gap-1.5 shrink-0">
              <button
                class="px-2 py-0.5 text-[11px] rounded-md bg-slate-100 text-slate-600 hover:bg-slate-200 cursor-pointer disabled:opacity-50 transition-colors"
                disabled={acting.has(c.name)}
                onclick={() => setEnabled(c, !c.enabled)}
              >{c.enabled ? 'Pause' : 'Resume'}</button>
              <button
                class="px-2 py-0.5 text-[11px] rounded-md {confirmRemove === c.name
                  ? 'bg-red-600 text-white hover:bg-red-500'
                  : 'bg-slate-100 text-slate-600 hover:bg-slate-200'} cursor-pointer disabled:opacity-50 transition-colors"
                disabled={acting.has(c.name)}
                onclick={() => removeCompany(c)}
              >{confirmRemove === c.name ? 'Confirm remove?' : 'Remove'}</button>
              <button
                class="px-2 py-0.5 text-[11px] rounded-md bg-slate-800 text-white hover:opacity-90 cursor-pointer disabled:opacity-50 transition-colors"
                disabled={acting.has(c.name)}
                onclick={() => sweepNow(c)}
              >{acting.has(c.name) ? 'Sweeping…' : 'Sweep now'}</button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

{#if undo}
  <div role="status" aria-live="polite" class="fixed bottom-6 left-1/2 -translate-x-1/2 bg-slate-800 dark:bg-slate-700 text-white px-4 py-3 rounded-xl shadow-lg text-sm z-50 flex gap-3 items-center">
    <span>Removed {undo.entry.company || undo.entry.name}</span>
    <button class="underline underline-offset-2 text-emerald-300 hover:text-emerald-200 cursor-pointer" onclick={undoRemove}>Undo</button>
  </div>
{/if}
