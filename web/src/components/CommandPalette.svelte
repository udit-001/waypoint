<script>
  // WP-94 — command palette. ⌘K / Ctrl+K from anywhere.
  //
  // Two jobs, one input:
  //   - Jump: fuzzy-match one of 6 static destinations (Applications,
  //     Artifacts, Categories, Profile, AI Skills, Settings) and navigate.
  //   - Find: live FTS via `api.searchAll(q)` over jobs + artifacts.
  //
  // Mounted once in App.svelte so open/close state survives route changes.
  // The web UI is read-only — no action commands, no identifier fast-paths,
  // no submenu/prompt modes (those belong to lific, which mutates).

  import { tick } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  import { getCommandPalette } from '../stores/commandPalette.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import * as api from '../stores/api.svelte.js';
  import { skillLabel } from '../stores/skillMeta.js';

  const router = getRouter();
  const palette = getCommandPalette();

  // ── Static navigation destinations ───────────────────
  //
  // Applications is the home view (WP-95); everything else hangs off it.

  const NAV = [
    { title: 'Applications', route: '/applications', icon: 'briefcase' },
    { title: 'Artifacts', route: '/artifacts', icon: 'file-text' },
    { title: 'Categories', route: '/categories', icon: 'box' },
    { title: 'Profile', route: '/profile', icon: 'user' },
    { title: 'AI Skills', route: '/skills', icon: 'bot' },
    { title: 'Settings', route: '/settings', icon: 'settings' },
  ];

  const RESULTS_CAP = 8;

  // ── Open/close ───────────────────────────────────────
  //
  // `palette.open` lives in the shared `commandPalette` store so any
  // component (TopBar trigger, future rail items) can summon the palette
  // without prop-drilling. The component owns input/selection state.

  let query = $state('');
  let inputEl = $state(null);
  let listEl = $state(null);
  let selectedIdx = $state(0);

  function show() {
    query = '';
    selectedIdx = 0;
    results = [];
    palette.summon();
    void tick().then(() => inputEl?.focus());
  }

  function hide() {
    palette.dismiss();
  }

  function onWindowKeydown(e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      if (palette.open) hide();
      else void show();
      return;
    }
    if (!palette.open) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      hide();
    }
  }

  // ── FTS results ──────────────────────────────────────

  let results = $state([]);
  let searching = $state(false);
  let searchGen = 0;
  let debounce = null;

  function onInput() {
    selectedIdx = 0;
    if (debounce) clearTimeout(debounce);
    debounce = setTimeout(() => void runSearch(query), 120);
  }

  async function runSearch(q) {
    const gen = ++searchGen;
    const trimmed = q.trim();
    if (trimmed.length < 2) {
      results = [];
      searching = false;
      return;
    }
    searching = true;
    try {
      const data = await api.searchAll(trimmed);
      if (gen !== searchGen) return;
      results = (Array.isArray(data) ? data : []).slice(0, RESULTS_CAP);
    } catch {
      if (gen !== searchGen) return;
      results = [];
    } finally {
      if (gen === searchGen) searching = false;
    }
  }

  // ── Derived: navigate hits, flat selection list ──────

  let showResults = $derived(query.trim().length >= 2);

  // Case-insensitive substring match is enough for 6 destinations —
  // no need for fuzzy scoring like lific's multi-hundred catalog.
  let navHits = $derived.by(() => {
    const q = query.trim().toLowerCase();
    if (!q) return NAV;
    return NAV.filter((n) => n.title.toLowerCase().includes(q));
  });

  let flatItems = $derived.by(() => {
    const navs = navHits.map((n) => ({ t: 'nav', title: n.title, route: n.route, icon: n.icon }));
    const fts = results.map((r) => ({ t: 'fts', r }));
    return [...navs, ...fts];
  });

  // ── Selection + dispatch ─────────────────────────────

  function pickItem(it) {
    hide();
    if (it.t === 'nav') {
      router.navigate(it.route);
    } else {
      const r = it.r;
      router.navigate(r.type === 'job' ? `/job/${r.id}` : `/artifact/${r.id}`);
    }
  }

  function onInputKeydown(e) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIdx = Math.min(selectedIdx + 1, flatItems.length - 1);
      scrollSelectedIntoView();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIdx = Math.max(selectedIdx - 1, 0);
      scrollSelectedIntoView();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const it = flatItems[selectedIdx];
      if (it) pickItem(it);
    }
  }

  function scrollSelectedIntoView() {
    requestAnimationFrame(() => {
      listEl
        ?.querySelector(`[data-flat-idx="${selectedIdx}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    });
  }

  const typeIcon = { job: 'briefcase', artifact: 'file-text' };

  function resultSub(r) {
    if (r.type === 'job') return r.sub || 'Job';
    return skillLabel(r.sub) || 'Artifact';
  }
</script>

<svelte:window onkeydown={onWindowKeydown} />

{#if palette.open}
  <!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
  <div
    class="fixed inset-0 z-[100] bg-black/25 flex items-start justify-center pt-[14dvh] px-4"
    onclick={hide}
    role="presentation"
  >
    <!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
    <div
      class="w-full max-w-[560px] bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-xl shadow-[0_16px_48px_rgba(0,0,0,0.28)] overflow-hidden"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
    >
      <!-- Input row -->
      <div class="flex items-center gap-2.5 px-4 py-3 border-b border-slate-200 dark:border-slate-600">
        <span class="shrink-0 text-slate-400 dark:text-slate-500">{@html iconSvg('search', 15)}</span>
        <input
          bind:this={inputEl}
          bind:value={query}
          type="text"
          class="flex-1 bg-transparent border-0 outline-none text-sm text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500"
          placeholder="Jump or find… (Applications, or type to search)"
          oninput={onInput}
          onkeydown={onInputKeydown}
        />
        <kbd
          class="px-1.5 py-0.5 rounded border border-slate-200 dark:border-slate-600 bg-slate-100 dark:bg-slate-700 text-slate-400 dark:text-slate-500 font-mono text-[10px] leading-none shrink-0"
        >esc</kbd>
      </div>

      <!-- Results -->
      <div class="max-h-[420px] overflow-y-auto py-1.5" bind:this={listEl}>
        {#if flatItems.length === 0}
          <p class="px-4 py-6 text-center text-sm text-slate-400 dark:text-slate-500">
            {searching
              ? 'Searching…'
              : showResults
                ? `Nothing matches "${query.trim()}"`
                : 'Start typing or pick a destination'}
          </p>
        {:else}
          {#if navHits.length > 0}
            <div
              class="px-4 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500"
            >Navigate</div>
            {#each navHits as item, i (item.route)}
              <button
                class="w-full flex items-center gap-2.5 px-4 py-2 text-left transition-colors {i === selectedIdx
                  ? 'bg-slate-100 dark:bg-slate-700'
                  : 'hover:bg-slate-100 dark:hover:bg-slate-700'}"
                data-flat-idx={i}
                onclick={() => pickItem({ t: 'nav', route: item.route })}
                onmouseenter={() => { selectedIdx = i; }}
              >
                <span class="size-5 flex items-center justify-center shrink-0 text-slate-400 dark:text-slate-500">
                  {@html iconSvg(item.icon, 15)}
                </span>
                <span class="flex-1 text-sm text-slate-800 dark:text-slate-200 truncate">{item.title}</span>
                {#if i === selectedIdx}
                  <span class="text-[10px] text-slate-400 dark:text-slate-500 shrink-0 font-mono">↵</span>
                {/if}
              </button>
            {/each}
          {/if}

          {#if showResults && results.length > 0}
            <div
              class="px-4 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500"
            >Results</div>
            {#each results as r, i (r.type + '-' + r.id)}
              {@const flatIdx = navHits.length + i}
              <button
                class="w-full flex items-center gap-2.5 px-4 py-2 text-left transition-colors {flatIdx === selectedIdx
                  ? 'bg-slate-100 dark:bg-slate-700'
                  : 'hover:bg-slate-100 dark:hover:bg-slate-700'}"
                data-flat-idx={flatIdx}
                onclick={() => pickItem({ t: 'fts', r })}
                onmouseenter={() => { selectedIdx = flatIdx; }}
              >
                <span class="size-5 flex items-center justify-center shrink-0 text-slate-400 dark:text-slate-500">
                  {@html iconSvg(typeIcon[r.type] || 'file', 15)}
                </span>
                <span class="flex-1 min-w-0 flex flex-col gap-0.5">
                  <span class="w-full text-sm text-slate-800 dark:text-slate-200 truncate">
                    {r.title || 'Untitled'}
                  </span>
                  <span class="w-full text-xs text-slate-400 dark:text-slate-500 truncate">
                    {resultSub(r)}
                  </span>
                </span>
                {#if flatIdx === selectedIdx}
                  <span class="text-[10px] text-slate-400 dark:text-slate-500 shrink-0 font-mono">↵</span>
                {/if}
              </button>
            {/each}
          {/if}
        {/if}
      </div>
    </div>
  </div>
{/if}
