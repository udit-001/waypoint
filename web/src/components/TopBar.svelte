<script>
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  import { getPage } from '../stores/page.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { getCommandPalette } from '../stores/commandPalette.svelte.js';
  import { getLayout } from '../stores/layout.svelte.js';
  import { getChartsOpen } from '../stores/chartsOpen.svelte.js';
  import FilterModal from './FilterModal.svelte';
  import { iconSvg } from '../lib/icons.js';

  const router = getRouter();
  const page = getPage();
  const filter = getFilter();
  const palette = getCommandPalette();
  const layoutStore = getLayout();
  const chartsOpen = getChartsOpen();

  let isDark = $state(false);
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

  function toggleTheme() {
    const html = document.documentElement;
    const next = isDark ? 'light' : 'dark';
    html.dataset.theme = next;
    localStorage.setItem('jobtracker_theme', next);
    isDark = !isDark;
    var m = document.getElementById('theme-color');
    if (m) m.content = getComputedStyle(html).getPropertyValue('--color-slate-50').trim();
  }
</script>

<header class="flex items-center justify-between gap-4 min-h-10 px-6 py-1.5 bg-stone-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-600">
  <div class="flex items-center gap-4">
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
      {#if page.byline}
        <span class="text-xs text-slate-400 dark:text-slate-500 whitespace-nowrap shrink-0">{page.byline}</span>
      {/if}
    {/if}
  </div>

  <div class="flex items-center gap-2">
    {#if router.current.route === 'applications'}
      <!-- WP-95: List|Kanban segmented toggle, only on /applications.
           Matches the WP-93 prototype's inset-track + active-button style. -->
      <div class="flex items-center gap-0.5 p-0.5 rounded-md bg-slate-100 dark:bg-slate-700 shadow-[inset_0_1px_2px_rgba(0,0,0,0.10)]">
        <button
          class="px-2.5 py-1 rounded text-xs font-medium flex items-center gap-1.5 transition-colors {layoutStore.current === 'list' ? 'bg-white dark:bg-slate-600 text-slate-800 dark:text-slate-100 shadow-sm' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
          onclick={() => layoutStore.set('list')}
          aria-pressed={layoutStore.current === 'list'}
        >
          {@html iconSvg('list', 14, { duotone: false })}
          <span>List</span>
        </button>
        <button
          class="px-2.5 py-1 rounded text-xs font-medium flex items-center gap-1.5 transition-colors {layoutStore.current === 'kanban' ? 'bg-white dark:bg-slate-600 text-slate-800 dark:text-slate-100 shadow-sm' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
          onclick={() => layoutStore.set('kanban')}
          aria-pressed={layoutStore.current === 'kanban'}
        >
          {@html iconSvg('kanban', 14)}
          <span>Kanban</span>
        </button>
      </div>

      <!-- Chart visibility toggle — view-level control, same cluster as
           List/Kanban. Toggles the VelocityChart mount below the TopBar. -->
      <button
        class="flex items-center gap-1.5 h-7 px-2 rounded-md text-xs font-medium transition-colors cursor-pointer {chartsOpen.open ? 'bg-slate-700 dark:bg-slate-900 text-white' : 'text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600'}"
        onclick={() => chartsOpen.toggle()}
        aria-pressed={chartsOpen.open}
        title="Toggle velocity chart"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M3 3v18h18"/>
          <path d="m19 9-5 5-4-4-3 3"/>
        </svg>
        <span>Chart</span>
      </button>
    {/if}

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

    {#if router.current.route === 'applications'}
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
