<script>
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  import { iconSvg } from '../lib/icons.js';

  let { sidebarClosed, onToggleSidebar } = $props();
  const router = getRouter();

  let searchQuery = $state('');
  let isDark = $state(false);

  onMount(() => {
    isDark = document.documentElement.dataset.theme === 'dark';
  });

  const viewTitles = {
    dashboard: 'Dashboard',
    kanban: 'Kanban Board',
    table: 'Table View',
    categories: 'Categories',
    profile: 'Profile',
    skills: 'AI Integration',
    artifacts: 'Artifacts',
    settings: 'Settings',
    search: 'Search',
    job: 'Job Detail',
    artifact: 'Artifact Detail',
  };

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

<header class="flex items-center justify-between gap-4 min-h-10 px-6 py-1.5 bg-stone-50 border-b border-slate-200">
  <div class="flex items-center gap-4">
    <button
      class="p-1 rounded hover:bg-slate-200 text-slate-600 cursor-pointer"
      onclick={onToggleSidebar}
      title="Toggle Sidebar"
    >
      {@html iconSvg('menu', 20)}
    </button>
    <h2 class="text-lg font-semibold text-slate-800 whitespace-nowrap">{viewTitles[router.current.route] || 'Dashboard'}</h2>
  </div>

  <div class="flex items-center gap-2">
    <div class="flex items-center bg-slate-100 rounded-lg px-2">
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Search jobs & artifacts... (/)"
        class="bg-transparent border-none outline-none w-56 py-1.5 px-2 text-sm text-slate-700 placeholder-slate-400 focus:w-72 transition-all"
        onkeydown={(e) => { if (e.key === 'Enter') handleSearch(); if (e.key === 'Escape') e.target.blur(); }}
      />
      {#if searchQuery}
        <button
          class="text-slate-400 hover:text-slate-600 cursor-pointer"
          onclick={() => searchQuery = ''}
        >×</button>
      {/if}
    </div>
    <button
      class="p-1.5 rounded hover:bg-slate-200 text-slate-600 cursor-pointer"
      onclick={toggleTheme}
      title="Toggle Theme"
    >
      {@html iconSvg(isDark ? 'sun' : 'moon', 18)}
    </button>
  </div>
</header>
