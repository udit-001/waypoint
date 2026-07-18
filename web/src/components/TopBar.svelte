<script>
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import { getRouter } from '../stores/router.svelte.js';
  import { getPage } from '../stores/page.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { getCommandPalette } from '../stores/commandPalette.svelte.js';
  import FilterModal from './FilterModal.svelte';
  import { iconSvg } from '../lib/icons.js';
  import { skillLabel } from '../stores/skillMeta.js';
  import * as api from '../stores/api.svelte.js';

  let { sidebarClosed, onToggleSidebar } = $props();
  const router = getRouter();
  const page = getPage();
  const filter = getFilter();
  const palette = getCommandPalette();

  let searchQuery = $state('');
  let isDark = $state(false);
  let results = $state([]);
  let showDropdown = $state(false);
  let debounceTimer = $state(null);
  let showInstallBtn = $state(false);
  let deferredPrompt = $state(null);

  onMount(() => {
    isDark = document.documentElement.dataset.theme === 'dark';

    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault();
      deferredPrompt = e;
      showInstallBtn = true;
    });
    window.addEventListener('appinstalled', dismissInstallPrompt);
  });

  function dismissInstallPrompt() {
    deferredPrompt = null;
    showInstallBtn = false;
  }

  function handleInstall() {
    if (!deferredPrompt) return;
    deferredPrompt.prompt();
    deferredPrompt.userChoice.then(dismissInstallPrompt);
  }

  function onSearchInput() {
    clearTimeout(debounceTimer);
    const q = searchQuery.trim();
    if (q.length < 2) {
      showDropdown = false;
      results = [];
      return;
    }
    debounceTimer = setTimeout(async () => {
      if (q.length < 2) return;
      try {
        const data = await api.searchAll(q);
        results = Array.isArray(data) ? data : [];
        showDropdown = results.length > 0;
      } catch {
        results = [];
        showDropdown = false;
      }
    }, 250);
  }

  function handleKeydown(e) {
    if (e.key === 'Enter') {
      if (searchQuery.length >= 2) {
        showDropdown = false;
        router.navigate('/table');
      }
    }
    if (e.key === 'Escape') {
      showDropdown = false;
      e.target.blur();
    }
  }

  function clearSearch() {
    searchQuery = '';
    results = [];
    showDropdown = false;
  }

  function goToResult(type, id) {
    showDropdown = false;
    searchQuery = '';
    results = [];
    router.navigate(type === 'job' ? '/job/' + id : '/artifact/' + id);
  }

  function toggleTheme() {
    const html = document.documentElement;
    const next = isDark ? 'light' : 'dark';
    html.dataset.theme = next;
    localStorage.setItem('jobtracker_theme', next);
    isDark = !isDark;
    var m = document.getElementById('theme-color');
    if (m) m.content = getComputedStyle(html).getPropertyValue('--color-slate-50').trim();
  }

  const typeLabels = { job: 'briefcase', artifact: 'file-text' };
</script>

<header class="flex items-center justify-between gap-4 min-h-10 px-6 py-1.5 bg-stone-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-600">
  <div class="flex items-center gap-4 min-w-0">
    <button
      class="p-2 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 cursor-pointer inline-flex items-center justify-center min-w-[40px] min-h-[40px] transition-colors"
      onclick={onToggleSidebar}
      title="Toggle Sidebar"
    >
      {@html iconSvg('menu', 20)}
    </button>

    {#if page.breadcrumbs.length > 0}
      <nav class="flex items-center gap-1.5 text-sm min-w-0">
        {#each page.breadcrumbs as crumb, i}
          {#if i > 0}
            <svg class="text-slate-300 dark:text-slate-500 shrink-0" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>
          {/if}
          {#if i < page.breadcrumbs.length - 1}
            <button
              class="text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer bg-transparent border-none p-0 text-sm truncate max-w-[200px]"
              onclick={crumb.action}
            >{crumb.label}</button>
          {:else}
            <span class="text-slate-800 dark:text-slate-200 font-semibold truncate min-w-0">{crumb.label}</span>
          {/if}
        {/each}
      </nav>
    {:else}
      <h2 class="text-lg font-semibold text-slate-800 dark:text-slate-200 whitespace-nowrap">{page.title}</h2>
    {/if}
  </div>

  <div class="flex items-center gap-2">
    <button
      class="flex items-center gap-2 border border-slate-200 dark:border-slate-600 rounded-md px-3 py-1.5 bg-white dark:bg-slate-700 text-xs text-slate-500 dark:text-slate-400 cursor-pointer hover:border-slate-400 dark:hover:border-slate-500 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
      onclick={() => palette.summon()}
      title="Open command palette (⌘K)"
      aria-label="Open command palette"
    >
      {@html iconSvg('search', 14)}
      <span>Search…</span>
      <kbd class="text-[10px] px-1.5 py-px rounded border border-slate-200 dark:border-slate-600 bg-slate-100 dark:bg-slate-800 text-slate-400 dark:text-slate-500 font-sans">⌘K</kbd>
    </button>

    <div class="relative">
      <div class="flex items-center bg-slate-100 dark:bg-slate-700 rounded-lg px-2">
        <input
          type="text"
          bind:value={searchQuery}
          oninput={onSearchInput}
          placeholder="Search jobs & artifacts... (/)"
          class="bg-transparent border-none outline-none w-56 py-1.5 px-2 text-sm text-slate-700 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-400/40 rounded-lg"
          onkeydown={handleKeydown}
          onblur={() => setTimeout(() => { showDropdown = false; }, 200)}
        />
        {#if searchQuery}
          <button
            class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer"
            onclick={clearSearch}
          >×</button>
        {/if}
      </div>

      {#if showDropdown && results.length > 0}
        <div in:fly={{y: -4, duration: 140}} class="absolute top-full right-0 mt-1 w-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg shadow-lg z-50 max-h-72 overflow-y-auto" style="transform-origin: top right">
          {#each results as r}
            <button
              class="w-full text-left px-3 py-2 text-sm hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer bg-transparent border-none flex items-center gap-2 transition-colors"
              onmousedown={() => goToResult(r.type, r.id)}
            >
              <span class="shrink-0 flex items-center">{@html iconSvg(typeLabels[r.type], 16)}</span>
              <span class="flex-1 min-w-0 truncate">{r.title || 'Untitled'}</span>
              <span class="text-xs text-slate-400 dark:text-slate-500 shrink-0">{r.type === 'job' ? r.sub : skillLabel(r.sub)}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    {#if router.current.route === 'dashboard' || router.current.route === 'kanban' || router.current.route === 'table'}
      <button
        class="flex items-center gap-1.5 h-7 px-2 rounded-md text-xs font-medium text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 cursor-pointer transition-colors"
        onclick={() => filter.toggle()}
        title="Filter applications"
        aria-label="Open filter"
        aria-expanded={filter.open}
      >
        {@html iconSvg('sliders', 14)}
        <span>Filter</span>
        {#if filter.activeCount > 0}
          <span
            class="grid place-items-center min-w-[18px] h-[18px] px-1 rounded-full bg-slate-700 dark:bg-slate-900 text-white font-mono text-[10px] leading-none tabular-nums"
          >{filter.activeCount}</span>
        {/if}
      </button>
      <FilterModal />
    {/if}
    {#if showInstallBtn}
      <button
        class="p-2.5 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 cursor-pointer inline-flex items-center justify-center min-w-[40px] min-h-[40px] transition-colors"
        onclick={handleInstall}
        title="Install app"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
      </button>
    {/if}
    <button
      class="p-2.5 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 cursor-pointer inline-flex items-center justify-center min-w-[40px] min-h-[40px] transition-colors"
      onclick={toggleTheme}
      title="Toggle Theme"
    >
      {@html iconSvg(isDark ? 'sun' : 'moon', 18)}
    </button>
  </div>
</header>
