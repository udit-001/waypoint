<script>
  // Matches — the autopilot review queue, WP-134.
  //
  // Surfaces shortlisted postings (score + reasons from zen curation)
  // for a keep/discard decision: Add → promoted into Applications,
  // Dismiss → removed from the queue. Deliberately mirrors the
  // Applications list grammar (full-bleed rows, sticky group headers,
  // byline in the TopBar) so both queue views read as one product:
  //   - groups are score bands (Strong/Good/Fair/Weak), not statuses
  //   - the score dot at row start plays the role of the status dot
  //   - expand-in-place detail carries reasons/description/actions
  //
  // Post-action: rows are removed optimistically; a confirmation toast
  // (no Undo — promote/dismiss aren't reversible through the API yet;
  // real undo needs a DELETE endpoint, filed as follow-up).

  import { onMount, onDestroy } from 'svelte';
  import { fly } from 'svelte/transition';
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { formatDateShort, formatDateFull } from '../lib/format.js';
  import { candidateBoardLabel, pendingCandidates } from '../lib/discovery.js';
  import { renderMarkdown } from '../lib/markdown.js';
  import { subscribeLive } from '../lib/live.js';
  import Skeleton from '../components/Skeleton.svelte';
  import * as api from '../stores/api.svelte.js';
  import { getRouter } from '../stores/router.svelte.js';

  const router = getRouter();

  // Score bands — the found-jobs counterpart to STATUSES. Color is the
  // canonical hue (hex, like STATUS_META); the label is the group
  // header vocabulary.
  const BANDS = [
    { id: 'strong', label: 'Strong', min: 80, color: '#10b981' },
    { id: 'good',   label: 'Good',   min: 60, color: '#f59e0b' },
    { id: 'fair',   label: 'Fair',   min: 40, color: '#f97316' },
    { id: 'weak',   label: 'Weak',   min: 0,  color: '#94a3b8' },
  ];

  function bandFor(score) {
    return BANDS.find(b => score >= b.min) || BANDS[BANDS.length - 1];
  }

  let queue = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let selected = $state(new Set());
  let toast = $state(null);
  let toastTimer = $state(null);
  let autopilotData = $state(null);
  let expandedUrl = $state(null);
  let collapsedGroups = $state(new Set());
  let firstRender = true;
  // In-flight guards: per-URL acting set stops double-fire from rapid
  // clicks (promote is idempotent server-side, but a second click still
  // wastes a POST and double-fires the toast); busy flag stops batch
  // loops from being re-entered while a previous loop is still running.
  const acting = new Set();
  let batchBusy = $state(false);

  onMount(async () => {
    await loadQueue();
    await loadAutopilot();
    await loadCandidates();
    firstRender = false;

    // Live-sync (WP-144): a cycle's sweep/curate moves the queue and the
    // run ledger — refetch both without flipping the loading skeleton.
    unsubLive = subscribeLive('matches', () => {
      loadQueue(true);
    });
    unsubLiveRuns = subscribeLive('runs', () => {
      loadAutopilot();
      loadQueue(true);
    });
    // Discovery decisions (WP-154) move candidates across surfaces —
    // another tab's add/dismiss drops the row here too.
    unsubLiveCands = subscribeLive('candidates', () => {
      loadCandidates(true);
    });
  });

  // ── Discovery band (WP-154) ──────────────────────────
  let cands = $state([]);
  let candActing = $state(new Set());
  let candError = $state(null);
  let unsubLiveCands = null;

  async function loadCandidates(silent = false) {
    try {
      await api.candidates.ensure();
      if (silent) await api.candidates.refresh();
      cands = pendingCandidates(api.candidates.value);
    } catch { /* band is optional chrome — stay quiet on failure */ }
  }

  async function addCompany(c) {
    if (candActing.has(c.id)) return;
    candActing.add(c.id);
    try {
      const res = await api.addCandidate(c.id);
      cands = cands.filter(x => x.id !== c.id);
      showToast('Added company', { result: { title: c.name, company: res.meta?.detail || '' }, link: '/companies' });
    } catch (e) { candError = e.message; } finally {
      candActing.delete(c.id);
    }
  }

  async function dismissCompany(c) {
    if (candActing.has(c.id)) return;
    candActing.add(c.id);
    try {
      await api.dismissCandidate(c.id);
      cands = cands.filter(x => x.id !== c.id);
      showToast('Dismissed', { result: { title: c.name, company: 'won\'t be suggested again' } });
    } catch (e) { candError = e.message; } finally {
      candActing.delete(c.id);
    }
  }

  let unsubLive = null;
  let unsubLiveRuns = null;
  onDestroy(() => {
    if (toastTimer) clearTimeout(toastTimer);
    if (unsubLive) unsubLive();
    if (unsubLiveRuns) unsubLiveRuns();
    if (unsubLiveCands) unsubLiveCands();
  });

  async function loadQueue(silent = false) {
    if (!silent) {
      loading = true;
      error = null;
    }
    try {
      const previousSelection = selected;
      if (silent) {
        await api.postings.refresh();
      } else {
        await api.postings.ensure();
      }
      queue = (api.postings.value || []).filter(p => p.status === 'shortlisted');
      // Keep the reviewer's selection across live refetches — a background
      // sweep landing mid-review must not silently drop checked rows.
      // Selection only shrinks when its rows actually left the queue.
      const urls = new Set(queue.map(q => q.result.url));
      selected = new Set([...previousSelection].filter(u => urls.has(u)));
    } catch (e) {
      if (!silent) error = e.message;
    } finally {
      if (!silent) loading = false;
    }
  }

  async function loadAutopilot() {
    try {
      await api.autopilot.ensure();
      autopilotData = api.autopilot.value;
    } catch { /* byline degrades to counts */ }
  }

  // Header (via setPage): three hierarchy levels — title (identity),
  // status chip (live state, one notch louder), byline (quiet facts).
  // Chip tones: running now = live pulse, degraded/blocked = warn with
  // a link to the fix, off = quiet link to Settings. Idle (next run
  // known, scoring on) needs no chip — the byline's "next run ~Xh"
  // carries it at fact level.
  let byline = $derived.by(() => {
    const parts = [];
    if (queue.length > 0) parts.push(queue.length === 1 ? '1 match' : `${queue.length} matches`);
    if (autopilotData) {
      const run = autopilotData.lastRun;
      if (autopilotData.enabled && run && run.finishedAt) {
        const bits = [];
        if (run.postingsNew > 0) bits.push(`${run.postingsNew} new`);
        if (run.postingsErrored > 0) bits.push(`${run.postingsErrored} errored`);
        if (bits.length > 0) parts.push(`last run ${bits.join(', ')}`);
      }
      // Healthy idle: the "when" question at fact level, no chip.
      if (autopilotData.enabled && autopilotData.zenKeySet) {
        parts.push(`next run ~${nextCadence()}`);
      }
    }
    return parts.join(' · ');
  });

  let status = $derived.by(() => {
    if (!autopilotData) return null;
    const run = autopilotData.lastRun;
    if (!autopilotData.enabled) {
      return { label: 'off', tone: 'off', href: '/settings', title: 'Autopilot is off — enable in Settings' };
    }
    if (run && !run.finishedAt) {
      const ageMs = Date.now() - new Date(run.startedAt).getTime();
      if (ageMs < 2 * 3600e3) {
        return { label: 'running now', tone: 'live', title: 'An autopilot run is in progress' };
      }
      return { label: 'last run interrupted', tone: 'warn', href: '/settings', title: 'The last run did not finish — details in Settings' };
    }
    if (!autopilotData.zenKeySet) {
      return { label: 'scoring off — no Zen key', tone: 'warn', href: '/settings', title: 'Add your Zen API key in Settings for scored matches' };
    }
    return null;
  });

  $effect(() => { setPage({ title: 'Matches', byline, status }); });

  function nextCadence() {
    if (!autopilotData?.cadence) return 'a few hours';
    const h = autopilotData.cadence;
    if (h === 1) return '1 hour';
    if (h < 24) return `${h} hours`;
    return `${Math.round(h / 24)} days`;
  }

  function parseReasons(reasons) {
    if (!reasons) return [];
    let parsed;
    if (Array.isArray(reasons)) parsed = reasons;
    else { try { parsed = JSON.parse(reasons); } catch { return []; } }
    // New shape: {kind, field, text}. Legacy: plain strings — mapped
    // to a neutral chip so old verdicts still render.
    return parsed.map(r => {
      if (typeof r === 'string') return { kind: 'match', field: '', text: r };
      if (r && typeof r === 'object') {
        return { kind: r.kind === 'gap' ? 'gap' : 'match', field: r.field || '', text: r.text || '' };
      }
      return null;
    }).filter(Boolean);
  }

  // Field badge + tone classes for a reason chip.
  const FIELD_ICON = { role: 'briefcase', domain: 'box', level: 'star', location: 'target', company: 'user' };
  function reasonClass(r) {
    return r.kind === 'gap'
      ? 'border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300'
      : 'border-emerald-300 dark:border-emerald-700 bg-emerald-50 dark:bg-emerald-900/30 text-emerald-800 dark:text-emerald-300';
  }

  // ── Derived pipeline ────────────────────────────────
  // Score desc within the queue; groups are score bands, empty bands
  // hidden — same shape as Applications' status grouping.
  let sortedQueue = $derived(
    [...queue].sort((a, b) => {
      const sa = parseInt(a.result.metadata?.score || '0');
      const sb = parseInt(b.result.metadata?.score || '0');
      return sb - sa;
    })
  );
  let groups = $derived(
    BANDS
      .map(b => ({ band: b, items: sortedQueue.filter(p => bandFor(parseInt(p.result.metadata?.score || '0')).id === b.id) }))
      .filter(g => g.items.length > 0)
  );

  // ── Actions ─────────────────────────────────────────
  async function addOne(p) {
    if (acting.has(p.result.url)) return;
    acting.add(p.result.url);
    try {
      await api.promotePosting(p.result.url);
      queue = queue.filter(q => q.result.url !== p.result.url);
      selected = new Set([...selected].filter(u => u !== p.result.url));
      if (expandedUrl === p.result.url) expandedUrl = null;
      showToast('Added', p);
    } catch (e) { error = e.message; } finally {
      acting.delete(p.result.url);
    }
  }

  async function dismissOne(p) {
    if (acting.has(p.result.url)) return;
    acting.add(p.result.url);
    try {
      await api.dismissPosting(p.result.url);
      queue = queue.filter(q => q.result.url !== p.result.url);
      selected = new Set([...selected].filter(u => u !== p.result.url));
      if (expandedUrl === p.result.url) expandedUrl = null;
      showToast('Dismissed', p);
    } catch (e) { error = e.message; } finally {
      acting.delete(p.result.url);
    }
  }

  async function addSelected() {
    for (const url of [...selected]) {
      const p = queue.find(q => q.result.url === url);
      if (p) await addOne(p);
    }
    selected = new Set();
  }

  async function dismissSelected() {
    for (const url of [...selected]) {
      const p = queue.find(q => q.result.url === url);
      if (p) await dismissOne(p);
    }
    selected = new Set();
  }

  function toggleSelect(url) {
    const s = new Set(selected);
    if (s.has(url)) s.delete(url); else s.add(url);
    selected = s;
  }

  function toggleExpand(url) {
    expandedUrl = expandedUrl === url ? null : url;
  }

  function toggleGroup(id) {
    const next = new Set(collapsedGroups);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    collapsedGroups = next;
  }

  function showToast(action, posting) {
    if (toastTimer) clearTimeout(toastTimer);
    toast = { action, title: posting.result.title, company: posting.result.company, link: posting.link || null };
    toastTimer = setTimeout(() => { toast = null; }, 6000);
  }

  // ── Keyboard ────────────────────────────────────────
  function handleKeydown(e) {
    if (queue.length === 0) return;
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

    switch (e.key) {
      case 'j': case 'ArrowDown':
        e.preventDefault();
        if (expandedUrl) {
          const idx = queue.findIndex(q => q.result.url === expandedUrl);
          if (idx < queue.length - 1) expandedUrl = queue[idx + 1].result.url;
        } else if (queue.length > 0) {
          expandedUrl = queue[0].result.url;
        }
        break;
      case 'k': case 'ArrowUp':
        e.preventDefault();
        if (expandedUrl) {
          const idx = queue.findIndex(q => q.result.url === expandedUrl);
          if (idx > 0) expandedUrl = queue[idx - 1].result.url;
          else expandedUrl = null;
        }
        break;
      case 'x':
        e.preventDefault();
        if (expandedUrl) { const p = queue.find(q => q.result.url === expandedUrl); if (p) dismissOne(p); }
        break;
      case 'Enter':
        e.preventDefault();
        if (expandedUrl) { const p = queue.find(q => q.result.url === expandedUrl); if (p) addOne(p); }
        break;
      case 'Escape':
        e.preventDefault();
        expandedUrl = null;
        selected = new Set();
        break;
      case 'o':
        e.preventDefault();
        if (expandedUrl) { window.open(expandedUrl, '_blank'); }
        break;
    }
  }

  function stagger(i, opts) {
    const { y = 4, duration = 200, step = 25, cap = 8 } = opts || {};
    if (!firstRender) return { duration: 0 };
    return { y, duration, delay: Math.min(i, cap) * step };
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if cands.length > 0}
  <!-- Discovery band (WP-154): companies suggested by 'discover run',
       awaiting an add/dismiss decision. Sits above the match queue —
       a company decision widens what future sweeps even fetch. -->
  <div class="-mx-6 -mt-6 mb-6 border-b border-slate-200 dark:border-slate-700 bg-slate-50/60 dark:bg-slate-800/60">
    <div class="flex items-center gap-2 px-6 py-2 border-b border-slate-100 dark:border-slate-700">
      <span class="text-[10px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">Discovered companies</span>
      <span class="bg-slate-200 dark:bg-slate-700 text-slate-600 rounded-full px-1.5 py-0.5 text-[10px] font-medium tabular-nums">{cands.length}</span>
    </div>
    {#if candError}<p class="px-6 pt-2 text-xs text-amber-700">⚠ {candError}</p>{/if}
    {#each cands as c (c.id)}
      <div class="flex items-center gap-3 px-6 py-2 border-b border-slate-100 dark:border-slate-700 hover:bg-white/40 transition-colors">
        <div class="min-w-0 flex items-baseline gap-2">
          <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-200">{c.name}</span>
          {#if c.domain}<span class="text-xs text-slate-500 dark:text-slate-400 truncate">{c.domain}</span>{/if}
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          {#each c.boards as b (b.url)}
            <span class="px-1.5 py-px rounded bg-slate-100 dark:bg-slate-700 text-[10px] font-mono text-slate-600">{candidateBoardLabel(b)}</span>
          {/each}
        </div>
        <span class="ml-auto shrink-0 text-[11px] text-slate-500 dark:text-slate-400" title={`suggested by the "${c.facet}" facet`}>via {c.facet}</span>
        <div class="flex items-center gap-1.5 shrink-0">
          <button
            class="px-2.5 py-1 text-[11px] font-medium rounded-md bg-emerald-100 text-emerald-700 hover:bg-emerald-200 cursor-pointer disabled:opacity-50 transition-colors"
            disabled={candActing.has(c.id)}
            onclick={() => addCompany(c)}
          >Add company</button>
          <button
            class="px-2.5 py-1 text-[11px] font-medium rounded-md bg-slate-100 text-slate-600 hover:bg-slate-200 cursor-pointer disabled:opacity-50 transition-colors"
            disabled={candActing.has(c.id)}
            onclick={() => dismissCompany(c)}
          >Dismiss</button>
        </div>
      </div>
    {/each}
  </div>
{/if}

{#if loading}
  <!-- Loading: skeleton rows, same shape as the Applications list. -->
  <div class="-mx-6 -mt-6">
    {#each Array(6) as _, i}
      <div class="flex items-center gap-3 px-6 py-2.5 border-b border-slate-100 dark:border-slate-700">
        <Skeleton variant="circle" class="size-2.5 shrink-0" />
        <Skeleton class="h-4 flex-1" />
        <Skeleton class="h-4 w-20 shrink-0" />
        <Skeleton class="h-4 w-12 shrink-0" />
      </div>
    {/each}
  </div>
{:else if error}
  <div class="text-center py-16">
    <p class="text-red-600 dark:text-red-400 mb-4 text-sm">{error}</p>
    <button class="px-4 py-2 bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 rounded-lg text-sm cursor-pointer" onclick={loadQueue}>Retry</button>
  </div>
{:else if queue.length === 0}
  <div class="text-center py-20 text-slate-400 dark:text-slate-500">
    <div class="text-4xl mb-3 opacity-50 flex items-center justify-center">{@html iconSvg('target', 48)}</div>
    <p class="text-sm">No matches yet — autopilot puts new matches here for review.</p>
    {#if autopilotData?.enabled && !autopilotData.zenKeySet && autopilotData.lastRun?.finishedAt !== ''}
      <p class="text-xs mt-2 text-amber-600 dark:text-amber-400">
        Scoring is off — <a href="/settings" class="underline">add your Zen API key in Settings</a> so matches arrive scored.
      </p>
    {/if}
    <p class="text-xs mt-2 text-slate-400 dark:text-slate-600">Ask your assistant to run <code class="bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded text-[11px]">waypoint autopilot run</code> for an immediate pass.</p>
  </div>
{:else}
  <!-- ── LIST ────────────────────────────────────────── -->
  <!-- Full-bleed like the Applications list: break out of App.svelte's
       p-6, sticky band headers pin to the scrollport top, dense
       border-b rows. -->
  <div class="-mx-6 -mt-6">
    <!-- Bulk action strip: replaces the old in-page header. Add N /
         Dismiss N when rows are selected, Add all / Dismiss all when
         not. -->
    <div class="sticky -top-6 z-30 flex items-center justify-between gap-2 px-6 py-1.5 bg-stone-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-600">
      <span class="text-[11px] text-slate-500 dark:text-slate-400">
        {selected.size > 0 ? `${selected.size} selected` : 'Review queue'}
      </span>
      <div class="flex items-center gap-1.5">
        {#if selected.size > 0}
          <button class="px-2.5 py-1 text-[11px] font-medium bg-emerald-700 dark:bg-emerald-700 text-white rounded-md hover:bg-emerald-800 dark:hover:bg-emerald-800 cursor-pointer disabled:opacity-50" disabled={batchBusy} onclick={addSelected}>Add {selected.size}</button>
          <button class="px-2.5 py-1 text-[11px] font-medium bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-md hover:bg-slate-200 dark:hover:bg-slate-600 cursor-pointer disabled:opacity-50" disabled={batchBusy} onclick={dismissSelected}>Dismiss {selected.size}</button>
        {:else}
          <button class="px-2.5 py-1 text-[11px] font-medium bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-md hover:bg-slate-200 dark:hover:bg-slate-600 cursor-pointer disabled:opacity-50" disabled={batchBusy} onclick={() => { for (const p of [...queue]) addOne(p); }}>Add all</button>
          <button class="px-2.5 py-1 text-[11px] font-medium bg-slate-100 dark:bg-slate-700 text-slate-500 dark:text-slate-400 rounded-md hover:bg-slate-200 dark:hover:bg-slate-600 cursor-pointer disabled:opacity-50" disabled={batchBusy} onclick={() => { for (const p of [...queue]) dismissOne(p); }}>Dismiss all</button>
        {/if}
      </div>
    </div>

    {#each groups as g, gi (g.band.id)}
      <!-- Band header — same grammar as a status group header. -->
      <div
        class="sticky -top-6 z-20 flex items-center gap-2 px-6 py-1.5 bg-slate-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700 cursor-pointer select-none"
        onclick={() => toggleGroup(g.band.id)}
      >
        <svg
          class="text-slate-400 transition-transform {collapsedGroups.has(g.band.id) ? '-rotate-90' : ''}"
          width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
        ><polyline points="6 9 12 15 18 9"/></svg>
        <span class="size-2 rounded-full shrink-0" style="background: {g.band.color}"></span>
        <span class="text-[10px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">{g.band.label} match</span>
        <span class="bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-full px-1.5 py-0.5 text-[10px] font-medium tabular-nums">{g.items.length}</span>
        {#if g.band.id !== 'weak'}<span class="text-[10px] text-slate-400 dark:text-slate-500">≥{g.band.min}</span>{/if}
      </div>
      {#if !collapsedGroups.has(g.band.id)}
        {#each g.items as p, i (p.result.url)}
          {@const score = parseInt(p.result.metadata?.score || '0')}
          {@const band = bandFor(score)}
          {@const reasons = parseReasons(p.result.metadata?.reasons)}
          {@const isExpanded = expandedUrl === p.result.url}
          {@const isSelected = selected.has(p.result.url)}
          <div class="relative z-0" in:fly={stagger(i + gi * 4, { y: 4, duration: 200, step: 25, cap: 8 })}>
            <!-- Compact row: checkbox · score dot+num · title · company ·
                 location … date · chevron. Click expands. -->
            <div
              class="flex items-center gap-3 px-6 py-2 border-b border-slate-100 dark:border-slate-700 cursor-pointer transition-colors focus:outline-none focus-visible:bg-slate-50 dark:focus-visible:bg-slate-700/40 {isExpanded ? 'bg-slate-50 dark:bg-slate-700/40' : 'hover:bg-slate-50 dark:hover:bg-slate-700/40'}"
              tabindex="0"
              role="button"
              aria-expanded={isExpanded}
              onclick={() => toggleExpand(p.result.url)}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  e.stopPropagation();
                  toggleExpand(p.result.url);
                }
              }}
            >
              <button
                class="w-4 h-4 rounded border-2 shrink-0 flex items-center justify-center transition-colors cursor-pointer
                  {isSelected ? 'border-emerald-500 bg-emerald-500' : 'border-slate-300 dark:border-slate-500 hover:border-slate-400'}"
                aria-label={isSelected ? 'Deselect row' : 'Select row'}
                onclick={(e) => { e.stopPropagation(); toggleSelect(p.result.url); }}
              >
                {#if isSelected}
                  <svg viewBox="0 0 12 12" class="w-2.5 h-2.5 text-white" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 6l3 3 5-5" /></svg>
                {/if}
              </button>

              <span class="shrink-0 inline-flex items-center gap-1.5 tabular-nums" title="{band.label} match — {score}/100">
                <span class="size-2 rounded-full" style="background: {band.color}"></span>
                <span class="text-xs font-semibold" style="color: {band.color}">{score}</span>
              </span>

              <span class="flex-1 min-w-0 text-sm truncate text-slate-800 dark:text-slate-100">
                {p.result.title}
                <span class="text-slate-400 dark:text-slate-500 mx-1">·</span>
                <span class="text-slate-600 dark:text-slate-300">{p.result.company}</span>
                {#if p.result.location}
                  <span class="text-slate-400 dark:text-slate-500 mx-1">·</span>
                  <span class="text-slate-500 dark:text-slate-400">{p.result.location}</span>
                {/if}
              </span>

              <span class="hidden sm:block text-xs text-slate-400 dark:text-slate-500 shrink-0 w-[60px] text-right tabular-nums">{formatDateShort(p.result.date) || '—'}</span>

              <span class="shrink-0 text-slate-300 dark:text-slate-500 transition-transform {isExpanded ? 'rotate-180' : ''}">
                {@html iconSvg('chevron-down', 14)}
              </span>
            </div>

            <!-- Expanded detail: reasons, description, meta, actions. -->
            {#if isExpanded}
              <div class="px-6 pb-4 pt-3 border-b border-slate-100 dark:border-slate-700 bg-slate-50 dark:bg-slate-700/40">
                {#if p.result.metadata?.note}
                  <!-- Scout's note: the delta the row can't show (stack,
                       remit, dealbreaker). Rendered as prose above the
                       chips — it's the one thing to read before deciding. -->
                  <p class="text-xs text-slate-600 dark:text-slate-300 leading-relaxed mb-2">{p.result.metadata.note}</p>
                {/if}
                {#if reasons.length > 0}
                  <div class="flex flex-wrap gap-1.5 mb-3">
                    {#each reasons as r}
                      <span class="inline-flex items-center gap-1 text-[11px] border rounded-md px-2 py-0.5 max-w-full {reasonClass(r)}" title={r.field ? `${r.field}: ${r.text}` : r.text}>
                        {#if r.field && FIELD_ICON[r.field]}
                          {@html iconSvg(FIELD_ICON[r.field], 10, { duotone: false })}
                        {/if}
                        <span class="font-medium">{r.field || (r.kind === 'gap' ? 'gap' : 'fit')}</span>
                        {#if r.text}<span class="opacity-60">·</span><span class="truncate">{r.text}</span>{/if}
                      </span>
                    {/each}
                  </div>
                {/if}

                <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs mb-3">
                  <div>
                    <span class="text-slate-400 dark:text-slate-500 uppercase tracking-wide text-[10px] font-semibold">Score</span>
                    <p class="text-slate-700 dark:text-slate-200 mt-0.5">{score}/100 · {band.label} match</p>
                  </div>
                  <div>
                    <span class="text-slate-400 dark:text-slate-500 uppercase tracking-wide text-[10px] font-semibold">Location</span>
                    <p class="text-slate-700 dark:text-slate-200 mt-0.5">{p.result.location || 'Not specified'}</p>
                  </div>
                  <div>
                    <span class="text-slate-400 dark:text-slate-500 uppercase tracking-wide text-[10px] font-semibold">Posted</span>
                    <p class="text-slate-700 dark:text-slate-200 mt-0.5">{formatDateFull(p.result.date) || 'Unknown'}</p>
                  </div>
                  <div>
                    <span class="text-slate-400 dark:text-slate-500 uppercase tracking-wide text-[10px] font-semibold">Found</span>
                    <p class="text-slate-700 dark:text-slate-200 mt-0.5">{formatDateFull(p.first_seen)}</p>
                  </div>
                </div>

                {#if p.result.metadata?.overview}
                  <!-- Neutral LLM overview replaces the verbatim posting
                       body on this page (full text is one click away at
                       the source). -->
                  <div class="mb-3">
                    <span class="text-slate-400 dark:text-slate-500 uppercase tracking-wide text-[10px] font-semibold">Overview</span>
                    <p class="text-xs text-slate-600 dark:text-slate-300 mt-1 leading-relaxed">{p.result.metadata.overview}</p>
                  </div>
                {/if}

                <div class="flex items-center gap-2">
                  <a
                    href={p.result.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-slate-800 dark:bg-slate-200 text-white dark:text-slate-800 rounded-lg hover:opacity-90 transition-colors"
                    onclick={(e) => e.stopPropagation()}
                  >
                    {@html iconSvg('external-link', 12)}
                    View job posting
                  </a>
                  <button
                    class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-300 rounded-lg hover:bg-emerald-200 dark:hover:bg-emerald-800 dark:hover:text-emerald-200 transition-colors cursor-pointer"
                    onclick={(e) => { e.stopPropagation(); addOne(p); }}
                  >
                    {@html iconSvg('check', 12)}
                    Add to applications
                  </button>
                  <button
                    class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-300 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors cursor-pointer"
                    onclick={(e) => { e.stopPropagation(); dismissOne(p); }}
                  >
                    {@html iconSvg('x', 12)}
                    Dismiss
                  </button>
                </div>
              </div>
            {/if}
          </div>
        {/each}
      {/if}
    {/each}

    <!-- Keyboard hints -->
    <div class="mt-4 flex items-center justify-center gap-4 text-[10px] text-slate-400 dark:text-slate-500">
      <span><kbd class="px-1 py-0.5 bg-slate-100 dark:bg-slate-800 rounded">j</kbd>/<kbd class="px-1 py-0.5 bg-slate-100 dark:bg-slate-800 rounded">k</kbd> navigate</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 dark:bg-slate-800 rounded">enter</kbd> add</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 dark:bg-slate-800 rounded">x</kbd> dismiss</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 dark:bg-slate-800 rounded">o</kbd> open link</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 dark:bg-slate-800 rounded">esc</kbd> close</span>
    </div>
  </div>
{/if}

<!-- Confirmation toast (no Undo — promote/dismiss aren't reversible via
     the API; real undo needs a DELETE endpoint, follow-up). -->
{#if toast}
  <div role="status" aria-live="polite" class="fixed bottom-6 left-1/2 -translate-x-1/2 max-w-[calc(100vw-2rem)] bg-slate-800 dark:bg-slate-700 text-white px-4 py-3 rounded-xl shadow-lg text-sm z-50 flex gap-2 items-center min-w-0">
    <span class="shrink-0">{toast.action}:</span>
    <span class="truncate min-w-0">{toast.title}{toast.company ? ' — ' + toast.company : ''}</span>
    {#if toast.link}
      <a
        href={toast.link}
        class="shrink-0 underline underline-offset-2 text-emerald-300 hover:text-emerald-200"
        onclick={(e) => { e.preventDefault(); router.navigate(toast.link); toast = null; }}
      >Companies</a>
    {/if}
  </div>
{/if}

<style>
  /* Rendered-markdown prose inside the expanded description — mirrors
     JobDetail's notes-content rules, sized down for the queue row. */
  .description-content :global(h1),
  .description-content :global(h2),
  .description-content :global(h3),
  .description-content :global(h4) {
    font-weight: 600;
    line-height: 1.3;
    margin: 12px 0 6px;
  }
  .description-content :global(h1) { font-size: 0.9rem; }
  .description-content :global(h2) { font-size: 0.85rem; }
  .description-content :global(h3),
  .description-content :global(h4) { font-size: 0.8rem; }
  .description-content :global(h1:first-child),
  .description-content :global(h2:first-child),
  .description-content :global(h3:first-child) { margin-top: 0; }
  .description-content :global(p) { margin: 0 0 6px; }
  .description-content :global(ul),
  .description-content :global(ol) { margin: 0 0 6px; padding-left: 18px; }
  .description-content :global(li) { margin-bottom: 2px; }
  .description-content :global(a) { color: #2563eb; text-decoration: underline; }
  /* Theme is an attribute ([data-theme="dark"]), not a class — the
     Tailwind dark: variant is wired via @custom-variant to the same
     attribute. These hand-written rules must match that convention or
     they silently never apply (links rendered light-blue on the dark
     panel at 2.4:1 — WCAG fail). */
  :global([data-theme="dark"]) .description-content :global(a) { color: #93c5fd; }
  .description-content :global(code) {
    background: var(--color-slate-100);
    border-radius: 4px;
    padding: 1px 4px;
    font-size: 0.75rem;
  }
  :global([data-theme="dark"]) .description-content :global(code) { background: var(--color-slate-800); }
  .description-content :global(table) { border-collapse: collapse; margin: 6px 0; font-size: 0.7rem; }
  .description-content :global(th),
  .description-content :global(td) { text-align: left; border-bottom: 1px solid var(--color-slate-200); padding: 4px 8px; }
</style>
