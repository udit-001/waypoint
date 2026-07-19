<script>
  // WP-95 — unified Applications view. One component, two layouts:
  //
  //   /applications?layout=list    (default)  grouped single-line rows
  //   /applications?layout=kanban            status columns
  //
  // Replaces Dashboard + Table + Kanban. Shared across layouts:
  //   - data: api.jobs (loaded once on mount)
  //   - filter: applyFilter(jobs, filter) — the 5-dimension store from WP-96
  //   - default sort: most-recently-updated first (updatedAt desc, fallback id desc)
  //   - default group: by status (pipeline order)
  //
  // The old Dashboard's stat cards collapse into a byline carried via
  // setPage() — the TopBar renders "N total · M% response" next to the
  // page title. The velocity chart mounts below the TopBar (above the
  // list/kanban) when the TopBar's Chart toggle is on.

  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import { getRouter } from '../stores/router.svelte.js';
  import { setPage } from '../stores/page.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { getLayout } from '../stores/layout.svelte.js';
  import { getChartsOpen } from '../stores/chartsOpen.svelte.js';
  import * as api from '../stores/api.svelte.js';
  import { STATUSES } from '../lib/status.js';
  import { applyFilter } from '../lib/filter.js';
  import { urgencyFor } from '../lib/urgency.js';
  import { formatDateShort } from '../lib/format.js';
  import { iconSvg } from '../lib/icons.js';
  import Skeleton from '../components/Skeleton.svelte';
  import VelocityChart from '../components/VelocityChart.svelte';

  const router = getRouter();
  const filter = getFilter();
  const layoutStore = getLayout();
  const chartsOpen = getChartsOpen();

  let allJobs = $state([]);
  let loaded = $state(false);
  let firstRender = true;
  let copiedCli = $state(false);
  let collapsedGroups = $state(new Set());

  // Status dot colors — single source for both List dot + Kanban column
  // accent. Inline style so we don't need a new CSS variant per status.
  // (STATUS_STYLES in lib/status.js uses bg-/text- combinations aimed at
  // pill badges — fine for badges elsewhere, wrong shape for a 10px dot.)
  const STATUS_DOT_COLORS = {
    'Not Applied': '#94a3b8',
    'Applied':     '#5e81ac',
    'Offer':       '#a3be8c',
    'Rejected':    '#bf616a',
    'Withdrawn':   '#4c566a',
  };

  const URGENCY_TONE_CLASS = {
    red:   'text-red-600 dark:text-red-400',
    amber: 'text-amber-600 dark:text-amber-400',
    muted: 'text-slate-400 dark:text-slate-500',
  };

  // ── Sort ────────────────────────────────────────────
  // Most-recently-updated first. updatedAt may be empty on legacy rows
  // (pre-WP columns) — fall back to appliedDate, then '0' (id desc).
  function sortKey(job) {
    return job.updatedAt || job.appliedDate || '';
  }
  function comparator(a, b) {
    const ua = sortKey(a), ub = sortKey(b);
    if (ua !== ub) return ua < ub ? 1 : -1; // desc
    return b.id - a.id;
  }

  // ── Derived pipeline ────────────────────────────────
  let filteredJobs = $derived(applyFilter(allJobs, filter));
  let sortedJobs = $derived([...filteredJobs].sort(comparator));
  let groups = $derived(
    STATUSES
      .map(s => ({ status: s, jobs: sortedJobs.filter(j => j.status === s) }))
      .filter(g => g.jobs.length > 0)
  );

  // ── Byline (carried to TopBar via setPage) ──────────
  // Total + response rate. Computed over the FULL job set, not the
  // filtered view — the byline answers "how's the search going?" which
  // the filter would otherwise distort.
  let byline = $derived.by(() => {
    const total = allJobs.length;
    if (total === 0) return '';
    const applied = allJobs.filter(j => j.status === 'Applied').length;
    const offers  = allJobs.filter(j => j.status === 'Offer').length;
    const rejected = allJobs.filter(j => j.status === 'Rejected').length;
    const responseRate = applied > 0 ? Math.round((offers + rejected) / applied * 100) : 0;
    return `${total} total · ${responseRate}% response`;
  });

  onMount(async () => {
    setPage({ title: 'Applications', byline: byline });
    filter.sync();
    await api.jobs.ensure();
    allJobs = api.jobs.value || [];
    loaded = true;
    firstRender = false;
  });

  // Re-sync the page header as data loads / the byline shifts.
  $effect(() => { setPage({ title: 'Applications', byline: byline }); });

  // ── Row interactions ───────────────────────────────
  function showJob(id) { router.navigate('/job/' + id); }

  function toggleGroup(status) {
    const next = new Set(collapsedGroups);
    if (next.has(status)) next.delete(status);
    else next.add(status);
    collapsedGroups = next;
  }

  async function copyCli() {
    try {
      await navigator.clipboard.writeText('waypoint jobs add "Company" "Position"');
      copiedCli = true;
      setTimeout(() => { copiedCli = false; }, 1500);
    } catch { /* clipboard blocked */ }
  }

  function stagger(i, opts) {
    const { y = 4, duration = 200, step = 25, cap = 8 } = opts || {};
    if (!firstRender) return { duration: 0 };
    return { y, duration, delay: Math.min(i, cap) * step };
  }

  // Per-status column jobs for Kanban — same sortedJobs, sliced by status.
  function jobsByStatus(status) { return sortedJobs.filter(j => j.status === status); }
</script>

{#if !loaded}
  <!-- Loading: skeletons prevent layout shift when data lands. -->
  {#if layoutStore.current === 'kanban'}
    <div class="flex gap-4 pb-4 overflow-x-auto">
      {#each Array(3) as _}
        <div class="flex flex-col flex-1 min-w-[280px] max-w-[320px] bg-slate-50/50 dark:bg-slate-800/40 rounded-2xl border-t-2 border-slate-200 dark:border-slate-700 p-3">
          <div class="flex items-center justify-between px-2 pb-3">
            <Skeleton class="h-3 w-20" />
            <Skeleton variant="circle" class="size-5" />
          </div>
          <div class="flex flex-col gap-2">
            {#each Array(3) as _}
              <Skeleton variant="block" class="h-16 w-full" />
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <div class="-mx-6 -mt-6">
      {#each Array(8) as _, i}
        <div class="flex items-center gap-3 px-6 py-2.5 border-b border-slate-100 dark:border-slate-700">
          <Skeleton variant="circle" class="size-2.5 shrink-0" />
          <Skeleton class="h-4 flex-1" />
          <Skeleton class="h-4 w-16 shrink-0" />
          <Skeleton class="h-4 w-20 shrink-0" />
          <Skeleton class="h-4 w-12 shrink-0" />
        </div>
      {/each}
    </div>
  {/if}
{:else if allJobs.length === 0}
  <!-- First-time empty: no jobs at all. Centered CLI hint + copy. -->
  <div class="text-center py-20 text-slate-400 dark:text-slate-500">
    <div class="text-5xl mb-4 opacity-50 flex items-center justify-center">{@html iconSvg('list', 64)}</div>
    <h3 class="text-xl font-semibold text-slate-700 dark:text-slate-300 mb-2">Your applications appear here</h3>
    <p class="max-w-sm mx-auto mb-6 leading-relaxed text-sm">Use the CLI to add them:</p>
    <div class="relative inline-block">
      <button
        class="absolute top-2 right-2 px-2.5 py-1 rounded text-xs font-medium cursor-pointer transition-colors {copiedCli ? 'bg-emerald-100 text-emerald-700' : 'bg-white text-slate-600 hover:bg-slate-100 border border-slate-200'}"
        onclick={copyCli}
      >{copiedCli ? '✓ Copied' : 'Copy'}</button>
      <pre class="bg-slate-100 dark:bg-slate-800 px-5 py-3 rounded-lg text-sm font-mono text-slate-700 dark:text-slate-300">waypoint jobs add "Company" "Position"</pre>
    </div>
  </div>
{:else if filteredJobs.length === 0 && filter.any}
  <!-- Velocity chart: edge-to-edge, flush with TopBar. Optional (collapsed
       by default; toggled from the TopBar's Chart button). -->
  <div class="-mx-6 -mt-6">
    {#if chartsOpen.open}
      <VelocityChart jobs={allJobs} />
    {/if}
  </div>
  <!-- Filter-empty: jobs exist, but the current filter excludes them all. -->
  <div class="text-center py-20 text-slate-400 dark:text-slate-500">
    <div class="text-4xl mb-4 flex items-center justify-center">{@html iconSvg('filter', 48)}</div>
    <h3 class="text-lg font-semibold text-slate-600 dark:text-slate-300 mb-1">No applications match these filters</h3>
    <button
      class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400 underline cursor-pointer bg-transparent border-none p-0 mt-2"
      onclick={() => filter.clear()}
    >Clear all</button>
  </div>
{:else if layoutStore.current === 'kanban'}
  <!-- Velocity chart (same as list layout). -->
  <div class="-mx-6 -mt-6">
    {#if chartsOpen.open}
      <VelocityChart jobs={allJobs} />
    {/if}
  </div>
  <!-- ── KANBAN LAYOUT ──────────────────────────────── -->
  <div class="flex gap-4 min-h-[calc(100vh-12rem)] pb-4 pt-6 overflow-x-auto">
    {#each STATUSES as status}
      {@const colJobs = jobsByStatus(status)}
      <div class="flex flex-col flex-1 min-w-[280px] max-w-[320px] bg-slate-50/50 dark:bg-slate-800/40 rounded-2xl border-t-2 p-3" style="border-top-color: {STATUS_DOT_COLORS[status]}">
        <div class="flex items-center justify-between px-2 pb-3">
          <span class="text-xs font-semibold uppercase tracking-wide" style="color: {STATUS_DOT_COLORS[status]}">{status}</span>
          <span class="rounded-full px-2 py-0.5 text-xs font-medium tabular-nums" style="background: {STATUS_DOT_COLORS[status]}20; color: {STATUS_DOT_COLORS[status]}">{colJobs.length}</span>
        </div>
        <div class="flex flex-col gap-2 flex-1 min-h-[60px] overflow-y-auto">
          {#each colJobs as job, i (job.id)}
            {@const u = urgencyFor(job)}
            <div in:fly={stagger(i, { y: 4, duration: 220, step: 30, cap: 6 })}>
              <button
                class="w-full text-left bg-white dark:bg-slate-700 rounded-lg border border-slate-200 dark:border-slate-600 p-2.5 cursor-pointer hover:border-slate-400 dark:hover:border-slate-500 hover-safe:-translate-y-0.5 transition-transform"
                onclick={() => showJob(job.id)}
              >
                <div class="text-sm font-semibold text-slate-800 dark:text-slate-100 mb-0.5">{job.position}</div>
                <div class="text-xs text-slate-500 dark:text-slate-400">{job.company}{#if job.location}<span class="text-slate-400 dark:text-slate-500"> · {job.location}</span>{/if}</div>
                <div class="flex items-center justify-between gap-2 mt-2">
                  <span class="bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-300 rounded px-1.5 py-0.5 text-[10px] uppercase font-semibold">{job.category || 'Uncategorized'}</span>
                  {#if u.kind !== 'none'}
                    <span class="text-xs font-medium inline-flex items-center gap-1 {URGENCY_TONE_CLASS[u.tone]}">
                      {#if u.kind === 'stale'}
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                      {:else if u.kind === 'deadline'}
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                      {/if}
                      {u.label}
                    </span>
                  {/if}
                </div>
              </button>
            </div>
          {:else}
            <p class="text-xs text-slate-300 dark:text-slate-600 text-center py-4">No jobs</p>
          {/each}
        </div>
      </div>
    {/each}
  </div>
{:else}
  <!-- Velocity chart (same as other layouts). -->
  <div class="-mx-6 -mt-6">
    {#if chartsOpen.open}
      <VelocityChart jobs={allJobs} />
    {/if}
  </div>
  <!-- ── LIST LAYOUT ────────────────────────────────── -->
  <!-- Break out of the App.svelte p-6 so rows can go edge-to-edge and
       sticky group headers stick to the very top of the scroll area.
       No -mt-6 here: the chart wrapper above already pulled up, and the
       list follows in normal flow. Sticky headers use -top-6 to stick
       at the visible top (24px above the scrollport = content box top). -->
  <div class="-mx-6">
    {#each groups as g, gi (g.status)}
      <div
        class="sticky -top-6 z-20 flex items-center gap-2 px-6 py-1.5 bg-slate-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 cursor-pointer select-none"
        onclick={() => toggleGroup(g.status)}
      >
        <svg
          class="text-slate-400 transition-transform {collapsedGroups.has(g.status) ? '-rotate-90' : ''}"
          width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
        ><polyline points="6 9 12 15 18 9"/></svg>
        <span class="text-[10px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">{g.status}</span>
        <span class="bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-full px-1.5 py-0.5 text-[10px] font-medium tabular-nums">{g.jobs.length}</span>
      </div>
      {#if !collapsedGroups.has(g.status)}
        {#each g.jobs as job, i (job.id)}
          {@const u = urgencyFor(job)}
          {@const isClosed = job.status === 'Rejected' || job.status === 'Withdrawn'}
          <div class="relative z-0" in:fly={stagger(i + gi * 4, { y: 4, duration: 200, step: 25, cap: 8 })}>
            <button
              class="w-full flex items-center gap-3 px-6 py-2 text-left border-b border-slate-100 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-700/40 cursor-pointer transition-colors"
              onclick={() => showJob(job.id)}
            >
              <span
                class="w-2.5 h-2.5 rounded-full shrink-0"
                style="background: {STATUS_DOT_COLORS[job.status]}"
                aria-hidden="true"
              ></span>
              <span class="flex-1 min-w-0 text-sm truncate {isClosed ? 'line-through text-slate-400 dark:text-slate-500' : 'text-slate-800 dark:text-slate-100'}">
                {job.position}
                <span class="text-slate-400 dark:text-slate-500 mx-1">·</span>
                <span class={isClosed ? 'text-slate-500 dark:text-slate-400' : 'text-slate-600 dark:text-slate-300'}>{job.company}</span>
              </span>
              <span class="hidden sm:inline-block text-[10px] font-medium px-1.5 py-0.5 rounded-full border border-slate-200 dark:border-slate-600 text-slate-500 dark:text-slate-400 shrink-0">{job.category || 'Uncategorized'}</span>
              <span class="text-xs font-medium shrink-0 w-[90px] text-right {URGENCY_TONE_CLASS[u.tone]} tabular-nums">{u.label}</span>
              <span class="hidden sm:block text-xs text-slate-400 dark:text-slate-500 shrink-0 w-[60px] text-right tabular-nums">{formatDateShort(job.appliedDate) || '—'}</span>
            </button>
          </div>
        {/each}
      {/if}
    {/each}
  </div>
{/if}
