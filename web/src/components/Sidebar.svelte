<script>
  import { getRouter } from '../stores/router.svelte.js';
  import { iconSvg } from '../lib/icons.js';

  let { sidebarClosed, onToggle } = $props();
  const router = getRouter();

  let spinning = $state(false);

  function handleLogoClick(e) {
    e.preventDefault();
    if (router.current.route === 'applications') {
      // Spin feedback on Applications home
      spinning = true;
      setTimeout(() => { spinning = false; }, 200);
    } else {
      handleNav('applications');
    }
  }

  const navSections = [
    {
      title: 'Views',
      items: [
        { view: 'applications', label: 'Applications', icon: 'list' },
      ],
    },
    {
      title: 'Organize',
      items: [
        { view: 'categories', label: 'Categories', icon: 'box' },
        { view: 'profile', label: 'Profile', icon: 'user' },
      ],
    },
    {
      title: 'AI',
      items: [
        { view: 'skills', label: 'AI Integration', icon: 'bot' },
        { view: 'artifacts', label: 'Artifacts', icon: 'folder' },
      ],
    },
  ];

  function handleNav(view) {
    router.navigate('/' + view);
  }

  function isActive(view) {
    return router.current.route === view;
  }
</script>

<aside
  class="flex flex-col border-r border-slate-200 bg-slate-100 overflow-hidden h-full"
  style="width: 15rem; min-width: 15rem;"
>
  <div class="flex items-center gap-2.5 px-5 py-1.5 border-b border-slate-200">
    <a
      href="/"
      class="flex items-center gap-2 text-sm font-semibold text-slate-800 hover:text-slate-600 no-underline"
      onclick={handleLogoClick}
    >
      <svg class="shrink-0 transition-transform duration-200 ease-[var(--ease-out)] {spinning ? 'rotate-180' : ''}" viewBox="0 0 100 100" width="28" height="28" aria-hidden="true">
        <g fill="none" stroke="currentColor" stroke-linecap="round">
          <circle cx="50" cy="50" r="32" stroke-width="4"/>
          <circle cx="50" cy="50" r="16" stroke-width="1.5" stroke-dasharray="4 6" opacity="0.4"/>
          <circle cx="50" cy="18" r="4" fill="currentColor" stroke="none"/>
          <circle cx="82" cy="50" r="4" fill="currentColor" stroke="none"/>
          <circle cx="50" cy="82" r="4" fill="currentColor" stroke="none"/>
          <circle cx="18" cy="50" r="4" fill="currentColor" stroke="none"/>
          <polygon points="50,40 60,50 50,60 40,50" fill="currentColor" stroke="none"/>
          <path d="M 50 18 A 32 32 0 0 1 82 50" stroke-width="2.5" opacity="0.5"/>
          <circle cx="50" cy="50" r="2" fill="currentColor" stroke="none"/>
        </g>
      </svg>
      Waypoint
    </a>
  </div>

  <nav class="flex flex-col flex-1 p-2 overflow-y-auto">
    {#each navSections as section}
      <div class="mb-2">
        {#if section.title}
          <span class="block px-3 pt-3 pb-1 text-xs font-semibold uppercase tracking-wider text-slate-400">
            {section.title}
          </span>
        {/if}
        {#each section.items as item}
          <a
            href="/{item.view}"
            class="flex items-center gap-2 px-3 py-1.5 rounded text-sm no-underline cursor-pointer transition-colors {isActive(item.view) ? 'bg-slate-700 text-white font-medium' : 'text-slate-700 hover:bg-slate-200 hover:text-slate-900'}"
            onclick={(e) => { e.preventDefault(); handleNav(item.view); }}
          >
            <span class="w-5 text-center flex items-center justify-center">{@html iconSvg(item.icon, 18)}</span>
            {item.label}
          </a>
        {/each}
      </div>
    {/each}

    <div class="border-t border-slate-200 mt-auto pt-2">
      <a
        href="/settings"
        class="flex items-center gap-2 px-3 py-1.5 rounded text-sm no-underline cursor-pointer transition-colors {isActive('settings') ? 'bg-slate-700 text-white font-medium' : 'text-slate-700 hover:bg-slate-200'}"
        onclick={(e) => { e.preventDefault(); handleNav('settings'); }}
      >
        <span class="w-5 text-center flex items-center justify-center">{@html iconSvg('sliders', 18)}</span>
        Settings
      </a>
    </div>
  </nav>
</aside>
