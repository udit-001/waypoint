<script>
  import { onMount, onDestroy } from 'svelte';
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import Spinner from '../components/Spinner.svelte';
  import * as api from '../stores/api.svelte.js';

  let queue = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let selected = $state(new Set());
  let undoToast = $state(null);
  let undoTimer = $state(null);
  let autopilotData = $state(null);
  let stats = $state({ kept: 0, skipped: 0 });
  let expandedUrl = $state(null);

  onMount(async () => {
    setPage({ title: 'Found Jobs' });
    await loadQueue();
    await loadAutopilot();
  });

  onDestroy(() => {
    if (undoTimer) clearTimeout(undoTimer);
  });

  async function loadQueue() {
    loading = true;
    error = null;
    try {
      await api.postings.ensure();
      queue = (api.postings.value || []).filter(p => p.status === 'shortlisted');
      selected = new Set();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadAutopilot() {
    try {
      await api.autopilot.ensure();
      autopilotData = api.autopilot.value;
    } catch {}
  }

  function scoreColor(score) {
    if (score >= 80) return { bg: 'bg-emerald-50', ring: 'ring-emerald-400', text: 'text-emerald-700', label: 'Strong match' };
    if (score >= 60) return { bg: 'bg-amber-50', ring: 'ring-amber-400', text: 'text-amber-700', label: 'Good match' };
    if (score >= 40) return { bg: 'bg-orange-50', ring: 'ring-orange-400', text: 'text-orange-700', label: 'Partial match' };
    return { bg: 'bg-slate-50', ring: 'ring-slate-300', text: 'text-slate-600', label: 'Weak match' };
  }

  function parseReasons(reasons) {
    if (!reasons) return [];
    if (Array.isArray(reasons)) return reasons;
    try { return JSON.parse(reasons); } catch { return [String(reasons)]; }
  }

  function formatDate(dateStr) {
    if (!dateStr) return '';
    try {
      const d = new Date(dateStr + 'T00:00:00');
      return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    } catch { return dateStr; }
  }

  async function keepOne(p) {
    try {
      await api.promotePosting(p.result.url);
      queue = queue.filter(q => q.result.url !== p.result.url);
      stats = { ...stats, kept: stats.kept + 1 };
      showUndo('Kept', p);
    } catch (e) { error = e.message; }
  }

  async function skipOne(p) {
    try {
      await api.dismissPosting(p.result.url);
      queue = queue.filter(q => q.result.url !== p.result.url);
      stats = { ...stats, skipped: stats.skipped + 1 };
      showUndo('Skipped', p);
    } catch (e) { error = e.message; }
  }

  async function keepSelected() {
    for (const url of [...selected]) {
      const p = queue.find(q => q.result.url === url);
      if (p) await keepOne(p);
    }
    selected = new Set();
  }

  async function skipSelected() {
    for (const url of [...selected]) {
      const p = queue.find(q => q.result.url === url);
      if (p) await skipOne(p);
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

  function showUndo(action, posting) {
    if (undoTimer) clearTimeout(undoTimer);
    undoToast = { action, title: posting.result.title, company: posting.result.company };
    undoTimer = setTimeout(() => { undoToast = null; }, 5000);
  }

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
        if (expandedUrl) { const p = queue.find(q => q.result.url === expandedUrl); if (p) skipOne(p); }
        break;
      case 'Enter':
        e.preventDefault();
        if (expandedUrl) { const p = queue.find(q => q.result.url === expandedUrl); if (p) keepOne(p); }
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

  function nextCadence() {
    if (!autopilotData?.cadence) return 'a few hours';
    const h = autopilotData.cadence;
    if (h === 1) return '1 hour';
    if (h < 24) return `${h} hours`;
    return `${Math.round(h / 24)} days`;
  }

  // Sort by score descending.
  let sortedQueue = $derived(
    [...queue].sort((a, b) => {
      const sa = parseInt(a.result.metadata?.score || '0');
      const sb = parseInt(b.result.metadata?.score || '0');
      return sb - sa;
    })
  );
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="max-w-4xl mx-auto px-4">
  {#if loading}
    <Spinner text="Loading found jobs..." />
  {:else if error}
    <div class="text-center py-12">
      <p class="text-red-600 mb-4">{error}</p>
      <button class="px-4 py-2 bg-slate-200 rounded-lg text-sm cursor-pointer" onclick={loadQueue}>Retry</button>
    </div>
  {:else if queue.length === 0 && stats.kept === 0 && stats.skipped === 0}
    <div class="text-center py-16">
      <div class="text-5xl mb-4">✨</div>
      <h2 class="text-xl font-bold text-slate-800 mb-2">All caught up</h2>
      <p class="text-slate-500">New matches land here after autopilot curates them.</p>
      <p class="text-slate-400 text-sm mt-2">Run <code class="bg-slate-100 px-1.5 py-0.5 rounded text-xs">waypoint autopilot run</code> to curate the backlog.</p>
    </div>
  {:else}
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <div>
        <h1 class="text-lg font-bold text-slate-800">
          {queue.length} curated {queue.length === 1 ? 'job' : 'jobs'}
        </h1>
        {#if stats.kept > 0 || stats.skipped > 0}
          <p class="text-sm text-slate-400">{stats.kept} kept · {stats.skipped} skipped</p>
        {/if}
      </div>
      <div class="flex items-center gap-2">
        {#if selected.size > 0}
          <button class="px-3 py-1.5 text-xs font-medium bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 cursor-pointer" onclick={keepSelected}>
            Keep {selected.size}
          </button>
          <button class="px-3 py-1.5 text-xs font-medium bg-red-100 text-red-700 rounded-lg hover:bg-red-200 cursor-pointer" onclick={skipSelected}>
            Skip {selected.size}
          </button>
        {:else}
          <button class="px-3 py-1.5 text-xs font-medium bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200 cursor-pointer" onclick={() => { for (const p of [...queue]) keepOne(p); }}>
            Keep all
          </button>
          <button class="px-3 py-1.5 text-xs font-medium bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200 cursor-pointer" onclick={() => { for (const p of [...queue]) skipOne(p); }}>
            Skip all
          </button>
        {/if}
      </div>
    </div>

    <!-- Autopilot strip -->
    {#if autopilotData}
      <div class="flex items-center justify-between px-4 py-2.5 bg-slate-50 rounded-xl border border-slate-200 mb-4 text-sm">
        <div class="flex items-center gap-2">
          <span class="text-slate-400">{@html iconSvg('zap', 14)}</span>
          <span class="text-slate-600">
            {autopilotData.enabled ? `Autopilot on — next sweep in ~${nextCadence()}` : 'Autopilot off'}
          </span>
        </div>
        <button
          class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors cursor-pointer {autopilotData.enabled ? 'bg-slate-800' : 'bg-slate-300'}"
          onclick={async () => {
            try {
              const res = await fetch('/api/settings', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ autopilot_enabled: autopilotData.enabled ? 0 : 1 }),
              });
              if (res.ok) {
                const data = await res.json();
                autopilotData = { ...autopilotData, enabled: data.autopilotEnabled === 1 };
              }
            } catch {}
          }}
        >
          <span class="inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform {autopilotData.enabled ? 'translate-x-4' : 'translate-x-0.5'}" />
        </button>
      </div>
    {/if}

    <!-- Job cards -->
    <div class="space-y-3">
      {#each sortedQueue as p (p.result.url)}
        {@const score = parseInt(p.result.metadata?.score || '0')}
        {@const reasons = parseReasons(p.result.metadata?.reasons)}
        {@const sc = scoreColor(score)}
        {@const isExpanded = expandedUrl === p.result.url}
        {@const isSelected = selected.has(p.result.url)}

        <div class="border rounded-xl overflow-hidden transition-all {isSelected ? 'border-emerald-300 bg-emerald-50/50' : 'border-slate-200 bg-white hover:border-slate-300'}">
          <!-- Compact row (always visible) -->
          <div
            class="flex items-start gap-3 px-4 py-3 cursor-pointer"
            onclick={() => toggleExpand(p.result.url)}
          >
            <!-- Checkbox -->
            <button
              class="w-4 h-4 mt-0.5 rounded border-2 shrink-0 flex items-center justify-center transition-colors cursor-pointer
                {isSelected ? 'border-emerald-500 bg-emerald-500' : 'border-slate-300 hover:border-slate-400'}"
              onclick={(e) => { e.stopPropagation(); toggleSelect(p.result.url); }}
            >
              {#if isSelected}
                <svg viewBox="0 0 12 12" class="w-2.5 h-2.5 text-white" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 6l3 3 5-5" /></svg>
              {/if}
            </button>

            <!-- Score badge -->
            <div class="shrink-0 w-11 h-11 rounded-full {sc.bg} ring-2 {sc.ring} flex flex-col items-center justify-center">
              <span class="text-sm font-bold {sc.text} leading-none">{score}</span>
              <span class="text-[8px] {sc.text} opacity-70 leading-none mt-0.5">/100</span>
            </div>

            <!-- Main content -->
            <div class="flex-1 min-w-0">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <h3 class="font-semibold text-sm text-slate-800 leading-snug">{p.result.title}</h3>
                  <div class="flex items-center gap-1.5 mt-0.5 flex-wrap">
                    <span class="text-sm text-slate-600 font-medium">{p.result.company}</span>
                    {#if p.result.location}
                      <span class="text-slate-300">·</span>
                      <span class="text-xs text-slate-400">{p.result.location}</span>
                    {/if}
                    {#if p.result.date}
                      <span class="text-slate-300">·</span>
                      <span class="text-xs text-slate-400">{formatDate(p.result.date)}</span>
                    {/if}
                  </div>
                </div>
                <span class="shrink-0 text-[10px] font-medium {sc.text} {sc.bg} px-2 py-0.5 rounded-full">
                  {sc.label}
                </span>
              </div>

              <!-- Zen reasons (always visible) -->
              {#if reasons.length > 0}
                <div class="mt-2 flex flex-wrap gap-1.5">
                  {#each reasons as reason}
                    <span class="inline-flex items-center gap-1 text-[11px] text-slate-500 bg-slate-50 border border-slate-100 rounded-md px-2 py-0.5">
                      <span class="text-slate-300">{@html iconSvg('check-circle', 10)}</span>
                      {reason}
                    </span>
                  {/each}
                </div>
              {/if}
            </div>

            <!-- Expand indicator -->
            <div class="shrink-0 text-slate-300 mt-1 transition-transform {isExpanded ? 'rotate-180' : ''}">
              {@html iconSvg('chevron-down', 16)}
            </div>
          </div>

          <!-- Expanded details -->
          {#if isExpanded}
            <div class="px-4 pb-4 pt-1 border-t border-slate-100">
              <div class="grid grid-cols-2 gap-3 text-xs mb-3">
                <div>
                  <span class="text-slate-400 uppercase tracking-wide text-[10px] font-semibold">Company</span>
                  <p class="text-slate-700 mt-0.5">{p.result.company}</p>
                </div>
                <div>
                  <span class="text-slate-400 uppercase tracking-wide text-[10px] font-semibold">Location</span>
                  <p class="text-slate-700 mt-0.5">{p.result.location || 'Not specified'}</p>
                </div>
                <div>
                  <span class="text-slate-400 uppercase tracking-wide text-[10px] font-semibold">Posted</span>
                  <p class="text-slate-700 mt-0.5">{formatDate(p.result.date) || 'Unknown'}</p>
                </div>
                <div>
                  <span class="text-slate-400 uppercase tracking-wide text-[10px] font-semibold">Found</span>
                  <p class="text-slate-700 mt-0.5">{formatDate(p.first_seen)}</p>
                </div>
              </div>

              {#if p.result.description}
                <div class="mb-3">
                  <span class="text-slate-400 uppercase tracking-wide text-[10px] font-semibold">Description</span>
                  <p class="text-xs text-slate-600 mt-1 leading-relaxed whitespace-pre-line line-clamp-6">{p.result.description}</p>
                </div>
              {/if}

              <div class="flex items-center gap-2">
                <a
                  href={p.result.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-slate-800 text-white rounded-lg hover:bg-slate-700 transition-colors"
                  onclick={(e) => e.stopPropagation()}
                >
                  {@html iconSvg('external-link', 12)}
                  View job posting
                </a>
                <button
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-emerald-100 text-emerald-700 rounded-lg hover:bg-emerald-200 transition-colors cursor-pointer"
                  onclick={(e) => { e.stopPropagation(); keepOne(p); }}
                >
                  {@html iconSvg('check', 12)}
                  Keep
                </button>
                <button
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-red-50 text-red-600 rounded-lg hover:bg-red-100 transition-colors cursor-pointer"
                  onclick={(e) => { e.stopPropagation(); skipOne(p); }}
                >
                  {@html iconSvg('x', 12)}
                  Skip
                </button>
              </div>
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Keyboard hints -->
    <div class="mt-4 flex items-center justify-center gap-4 text-[10px] text-slate-400">
      <span><kbd class="px-1 py-0.5 bg-slate-100 rounded">j</kbd>/<kbd class="px-1 py-0.5 bg-slate-100 rounded">k</kbd> navigate</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 rounded">enter</kbd> expand/keep</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 rounded">x</kbd> skip</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 rounded">o</kbd> open link</span>
      <span><kbd class="px-1 py-0.5 bg-slate-100 rounded">esc</kbd> close</span>
    </div>
  {/if}

  <!-- Undo toast -->
  {#if undoToast}
    <div class="fixed bottom-6 left-1/2 -translate-x-1/2 bg-slate-800 text-white px-4 py-3 rounded-xl shadow-lg flex items-center gap-3 z-50">
      <span class="text-sm">{undoToast.action}: {undoToast.title} — {undoToast.company}</span>
      <button class="text-xs text-slate-300 hover:text-white underline cursor-pointer" onclick={() => { undoToast = null; }}>Undo</button>
    </div>
  {/if}
</div>
