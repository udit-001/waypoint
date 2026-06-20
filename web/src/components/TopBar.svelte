<script>
  import { navigate } from 'svelte-routing';

  let { sidebarClosed, onToggleSidebar } = $props();

  let searchQuery = $state('');

  function handleSearch() {
    if (searchQuery.length >= 2) {
      navigate('/table?q=' + encodeURIComponent(searchQuery));
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Enter') handleSearch();
    if (e.key === '/') {
      e.preventDefault();
      document.getElementById('topbar-search')?.focus();
    }
  }

  function toggleTheme() {
    const html = document.documentElement;
    const next = html.dataset.theme === 'dark' ? 'light' : 'dark';
    html.dataset.theme = next;
    localStorage.setItem('jobtracker_theme', next);
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<header class="flex items-center justify-between gap-4 min-h-14 px-6 py-3 bg-stone-50 border-b border-slate-200">
  <div class="flex items-center gap-4">
    <button
      class="p-1 rounded hover:bg-slate-200 text-slate-600 cursor-pointer"
      onclick={onToggleSidebar}
      title="Toggle Sidebar"
    >
      <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
        <path d="M2 4.5h16M2 10h16M2 15.5h16" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round"/>
      </svg>
    </button>
    <h2 class="text-lg font-semibold text-slate-800 whitespace-nowrap">Dashboard</h2>
  </div>

  <div class="flex items-center gap-2">
    <div class="flex items-center bg-slate-100 rounded-lg px-2">
      <input
        id="topbar-search"
        type="text"
        bind:value={searchQuery}
        placeholder="Search jobs & artifacts... (/)"
        class="bg-transparent border-none outline-none w-56 py-1.5 px-2 text-sm text-slate-700 placeholder-slate-400 focus:w-72 transition-all"
        onkeydown={(e) => { if (e.key === 'Enter') handleSearch(); if (e.key === 'Escape') { e.target.blur(); } }}
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
      {#if typeof document !== 'undefined' && document.documentElement.dataset.theme === 'dark'}
        ☀
      {:else}
        ☾
      {/if}
    </button>
  </div>
</header>
