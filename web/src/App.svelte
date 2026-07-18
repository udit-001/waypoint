<script>
  import Sidebar from './components/Sidebar.svelte';
  import TopBar from './components/TopBar.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Dashboard from './views/Dashboard.svelte';
  import Kanban from './views/Kanban.svelte';
  import TableView from './views/TableView.svelte';
  import Categories from './views/Categories.svelte';
  import Profile from './views/Profile.svelte';
  import Skills from './views/Skills.svelte';
  import Artifacts from './views/Artifacts.svelte';
  import Settings from './views/Settings.svelte';
  import JobDetail from './views/JobDetail.svelte';
  import ArtifactDetail from './views/ArtifactDetail.svelte';
  import Search from './views/Search.svelte';
  import FilterSidebar from './components/FilterSidebar.svelte';
  import FilterBar from './components/FilterBar.svelte';
  import { getRouter } from './stores/router.svelte.js';
  import { setPage } from './stores/page.svelte.js';

  const router = getRouter();

  // Set correct page title immediately — before any view mounts
  const routeTitles = {
    dashboard: 'Dashboard', kanban: 'Kanban Board', table: 'Table View',
    categories: 'Categories', profile: 'Profile', skills: 'AI Integration',
    artifacts: 'Artifacts', settings: 'Settings', search: 'Search',
    job: 'Job Detail', artifact: 'Artifact',
  };
  setPage({ title: routeTitles[router.current.route] || 'Dashboard' });

  // Sidebar state persisted in localStorage
  let sidebarClosed = $state(
    typeof localStorage !== 'undefined'
      ? localStorage.getItem('jobtracker_sidebar_closed') === 'true'
      : false
  );

  function toggleSidebar() {
    sidebarClosed = !sidebarClosed;
    localStorage.setItem('jobtracker_sidebar_closed', sidebarClosed ? 'true' : 'false');
  }
</script>

<div id="app" class="grid h-screen overflow-hidden bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 transition-[grid-template-columns] duration-200 ease-[var(--ease-drawer)] will-change-[grid-template-columns] {sidebarClosed ? 'grid-cols-[0_1fr]' : 'grid-cols-[15rem_1fr]'}">
  <div class="overflow-hidden h-full">
    <Sidebar {sidebarClosed} onToggle={toggleSidebar} />
  </div>
  <main class="flex flex-col overflow-hidden">
    <TopBar {sidebarClosed} onToggleSidebar={toggleSidebar} />
    <FilterBar />
    <div class="flex-1 p-6 overflow-y-auto">
      {#if router.current.route === 'dashboard'}
        <Dashboard />
      {:else if router.current.route === 'kanban'}
        <Kanban />
      {:else if router.current.route === 'table'}
        <TableView />
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
      {:else if router.current.route === 'search'}
        <Search />
      {:else if router.current.route === 'job'}
        <JobDetail id={router.current.params.id} />
      {:else if router.current.route === 'artifact'}
        <ArtifactDetail id={router.current.params.id} />
      {/if}
    </div>
  </main>
  <FilterSidebar />
  <CommandPalette />
</div>
