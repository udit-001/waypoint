<script>
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  import { getPage } from '../stores/page.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { iconSvg } from '../lib/icons.js';

  let { sidebarClosed, onToggleSidebar } = $props();
  const router = getRouter();
  const page = getPage();
  const filter = getFilter();

  let searchQuery = $state('');
  let isDark = $state(false);

  onMount(() => {
    isDark = document.documentElement.dataset.theme === 'dark';
  });

  function handleSearch() {
    if (searchQuery.length >= 2) {
      router.navigate('/table');
    }
  }

  function toggleTheme() {
    const html = document.documentElement;
    const next = isDark ? 'light' : 'dark';
    html.dataset.theme = next;
    localStorage.setItem('jobtracker_theme', next);
    isDark = !isDark;
  }
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

  <div class="flex items-center gap-2">
    <div class="flex items-center bg-slate-100 dark:bg-slate-700 rounded-lg px-2">
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Search jobs & artifacts... (/)"
        class="bg-transparent border-none outline-none w-56 py-1.5 px-2 text-sm text-slate-700 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:w-72 transition-all"
        onkeydown={(e) => { if (e.key === 'Enter') handleSearch(); if (e.key === 'Escape') e.target.blur(); }}
      />
      {#if searchQuery}
        <button
          class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 cursor-pointer"
          onclick={() => searchQuery = ''}
        >×</button>
      {/if}
    </div>
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
