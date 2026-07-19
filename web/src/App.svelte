<script>
  import IconRail from './components/IconRail.svelte';
  import TopBar from './components/TopBar.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Applications from './views/Applications.svelte';
  import Categories from './views/Categories.svelte';
  import Profile from './views/Profile.svelte';
  import Skills from './views/Skills.svelte';
  import Artifacts from './views/Artifacts.svelte';
  import Settings from './views/Settings.svelte';
  import JobDetail from './views/JobDetail.svelte';
  import ArtifactDetail from './views/ArtifactDetail.svelte';
  import FilterBar from './components/FilterBar.svelte';
  import { getRouter } from './stores/router.svelte.js';
  import { setPage } from './stores/page.svelte.js';

  const router = getRouter();

  // Set correct page title immediately — before any view mounts
  const routeTitles = {
    applications: 'Applications', categories: 'Categories', profile: 'Profile',
    skills: 'AI Integration', artifacts: 'Artifacts', settings: 'Settings',
    job: 'Job Detail', artifact: 'Artifact',
  };
  setPage({ title: routeTitles[router.current.route] || 'Applications' });
</script>

<div id="app" class="grid h-screen overflow-hidden bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 grid-cols-[60px_1fr]">
  <IconRail />
  <main class="flex flex-col overflow-hidden">
    <TopBar />
    {#if router.current.route === 'applications'}
      <FilterBar />
    {/if}
    <div class="flex-1 p-6 overflow-y-auto">
      {#if router.current.route === 'applications'}
        <Applications />
      {:else if router.current.route === 'categories'}
        <Categories />
      {:else if router.current.route === 'profile'}
        <Profile />
      {:else if router.current.route === 'skills'}
        <Skills />
      {:else if router.current.route === 'artifacts'}
        <Artifacts />
      {:else if router.current.route === 'settings'}
        <Settings />
      {:else if router.current.route === 'job'}
        <JobDetail id={router.current.params.id} />
      {:else if router.current.route === 'artifact'}
        <ArtifactDetail id={router.current.params.id} />
      {/if}
    </div>
  </main>
  <CommandPalette />
</div>
