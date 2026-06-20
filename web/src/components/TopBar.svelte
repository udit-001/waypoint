<script>
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  import { getPage } from '../stores/page.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { skillLabel } from '../stores/skillMeta.js';
  import * as api from '../stores/api.svelte.js';

  let { sidebarClosed, onToggleSidebar } = $props();
  const router = getRouter();
  const page = getPage();
  const filter = getFilter();

  let searchQuery = $state('');
  let isDark = $state(false);
  let results = $state([]);
  let showDropdown = $state(false);
  let debounceTimer = $state(null);

  onMount(() => {
    isDark = document.documentElement.dataset.theme === 'dark';
  });

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
  }

  const typeLabels = { job: '💼 Job', artifact: '📄 Artifact' };
</script>

<header class="flex items-center justify-between gap-4 min-h-10 px-6 py-1.5 bg-stone-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-600">
  <div class="flex items-center gap-4">
    <button
      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 cursor-pointer"
      onclick={onToggleSidebar}
      title="Toggle Sidebar"
    >
      {@html iconSvg('menu', 20)}
    </button>

    {#if page.breadcrumbs.length > 0}
      <nav class="flex items-center gap-1.5 text-sm">
        {#each page.breadcrumbs as crumb, i}
          {#if i > 0}
            <span class="text-slate-300 dark:text-slate-500 mx-0.5">/</span>
          {/if}
          {#if i < page.breadcrumbs.length - 1}
            <button
              class="text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer bg-transparent border-none p-0 text-sm"
              onclick={crumb.action}
            >{crumb.label}</button>
          {:else}
            <span class="text-slate-800 dark:text-slate-200 font-semibold">{crumb.label}</span>
          {/if}
        {/each}
      </nav>
    {:else}
      <h2 class="text-lg font-semibold text-slate-800 dark:text-slate-200 whitespace-nowrap">{page.title}</h2>
    {/if}
  </div>

  <div class="flex items-center gap-2 relative">
    <div class="flex items-center bg-slate-100 dark:bg-slate-700 rounded-lg px-2 relative">
      <input
        type="text"
        bind:value={searchQuery}
        oninput={onSearchInput}
        placeholder="Search jobs & artifacts... (/)"
        class="bg-transparent border-none outline-none w-56 py-1.5 px-2 text-sm text-slate-700 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:w-72 transition-all"
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

    <!-- Autocomplete dropdown -->
    {#if showDropdown && results.length > 0}
      <div class="absolute top-full right-0 mt-1 w-80 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg shadow-lg z-50 max-h-72 overflow-y-auto">
        {#each results as r}
          <button
            class="w-full text-left px-3 py-2 text-sm hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer bg-transparent border-none flex items-center gap-2 transition-colors"
            onmousedown={() => goToResult(r.type, r.id)}
          >
            <span class="shrink-0">{r.type === 'job' ? '💼' : '📄'}</span>
            <span class="flex-1 min-w-0 truncate">{r.title || 'Untitled'}</span>
            <span class="text-xs text-slate-400 dark:text-slate-500 shrink-0">{r.type === 'job' ? r.sub : skillLabel(r.sub)}</span>
          </button>
        {/each}
      </div>
    {/if}

    <button
      class="p-1.5 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 cursor-pointer relative"
      onclick={filter.toggle}
      title="Toggle Filters"
    >
      {@html iconSvg('filter', 18)}
      {#if filter.category || filter.status}
        <span class="absolute -top-0.5 -right-0.5 w-2 h-2 bg-slate-700 rounded-full"></span>
      {/if}
    </button>
    <button
      class="p-1.5 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 cursor-pointer"
      onclick={toggleTheme}
      title="Toggle Theme"
    >
      {@html iconSvg(isDark ? 'sun' : 'moon', 18)}
    </button>
  </div>
</header>
