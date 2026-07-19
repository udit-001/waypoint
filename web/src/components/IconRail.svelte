<script>
  // WP-98 — 60px icon rail replacing the 15rem Sidebar.
  // Pure collapsed state, no expand toggle. Six icons in two groups:
  //   primary (top):    Applications, Artifacts
  //   secondary (end):  Categories, Profile, AI Skills, Settings
  // Hover any icon → flyout tooltip with the label (pointer:fine only;
  // touch users tap to navigate, the icon is its own affordance).
  // Logo navigates home (/applications) with a 200ms spin when already there.

  import { getRouter } from '../stores/router.svelte.js';
  import { iconSvg } from '../lib/icons.js';

  const router = getRouter();

  let spinning = $state(false);

  function handleLogoClick(e) {
    e.preventDefault();
    if (router.current.route === 'applications') {
      spinning = true;
      setTimeout(() => { spinning = false; }, 200);
    } else {
      router.navigate('/applications');
    }
  }

  const PRIMARY = [
    { view: 'applications', label: 'Applications', icon: 'list' },
    { view: 'artifacts', label: 'Artifacts', icon: 'file-text' },
  ];
  const SECONDARY = [
    { view: 'categories', label: 'Categories', icon: 'box' },
    { view: 'profile', label: 'Profile', icon: 'user' },
    { view: 'skills', label: 'AI Skills', icon: 'bot' },
    { view: 'settings', label: 'Settings', icon: 'settings' },
  ];

  function isActive(view) {
    return router.current.route === view;
  }
</script>

<aside
  class="flex flex-col border-r border-slate-200 dark:border-slate-600 bg-slate-100 dark:bg-slate-800 h-full relative z-30"
  style="width: 60px; min-width: 60px;"
>
  <!-- Logo (navigates home) -->
  <a
    href="/"
    class="flex items-center justify-center border-b border-slate-200 dark:border-slate-600 py-3 no-underline"
    onclick={handleLogoClick}
    aria-label="Waypoint home"
  >
    <div
      class="w-9 h-9 rounded-lg bg-slate-800 dark:bg-slate-600 text-white flex items-center justify-center font-bold text-sm shrink-0 transition-transform duration-200 ease-[var(--ease-out)] {spinning ? 'rotate-180' : ''}"
    >W</div>
  </a>

  <!-- Nav -->
  <nav class="flex flex-col flex-1 py-2 gap-0.5 px-2">
    {#each PRIMARY as item (item.view)}
      <a
        href="/{item.view}"
        class="rail-item flex items-center justify-center rounded-lg p-2 transition-colors {isActive(item.view)
          ? 'bg-slate-700 text-white'
          : 'text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 hover:text-slate-800 dark:hover:text-slate-100'}"
        onclick={(e) => { e.preventDefault(); router.navigate('/' + item.view); }}
        aria-label={item.label}
        aria-current={isActive(item.view) ? 'page' : undefined}
      >
        <span class="flex items-center justify-center">{@html iconSvg(item.icon, 20)}</span>
        <span class="tooltip">{item.label}</span>
      </a>
    {/each}

    <div class="flex-1 min-h-2"></div>

    <div class="border-t border-slate-200 dark:border-slate-600 pt-2 mt-1 flex flex-col gap-0.5">
      {#each SECONDARY as item (item.view)}
        <a
          href="/{item.view}"
          class="rail-item flex items-center justify-center rounded-lg p-2 transition-colors {isActive(item.view)
            ? 'bg-slate-700 text-white'
            : 'text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 hover:text-slate-800 dark:hover:text-slate-100'}"
          onclick={(e) => { e.preventDefault(); router.navigate('/' + item.view); }}
          aria-label={item.label}
          aria-current={isActive(item.view) ? 'page' : undefined}
        >
          <span class="flex items-center justify-center">{@html iconSvg(item.icon, 20)}</span>
          <span class="tooltip">{item.label}</span>
        </a>
      {/each}
    </div>
  </nav>
</aside>

<style>
  .rail-item {
    position: relative;
  }
  .tooltip {
    position: absolute;
    left: calc(100% + 10px);
    top: 50%;
    transform: translateY(-50%);
    background: #1e293b;
    color: white;
    padding: 4px 10px;
    border-radius: 4px;
    font-size: 12px;
    white-space: nowrap;
    opacity: 0;
    pointer-events: none;
    transition: opacity 120ms ease-out;
    z-index: 50;
  }
  /* Touch devices (no hover) never see the tooltip — they tap to navigate. */
  @media (hover: hover) and (pointer: fine) {
    .rail-item:hover .tooltip {
      opacity: 1;
    }
  }
</style>
