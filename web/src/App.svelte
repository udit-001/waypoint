<script>
  import { Router, Route, navigate } from 'svelte-routing';
  import Sidebar from './components/Sidebar.svelte';
  import TopBar from './components/TopBar.svelte';
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

  let sidebarClosed = $state(false);

  function toggleSidebar() {
    sidebarClosed = !sidebarClosed;
  }
</script>

<div id="app" class="flex h-screen overflow-hidden">
  <Sidebar {sidebarClosed} onToggle={toggleSidebar} />
  <main class="flex flex-col flex-1 overflow-hidden">
    <TopBar {sidebarClosed} onToggleSidebar={toggleSidebar} />
    <div class="flex-1 p-6 overflow-y-auto">
      <Router url="/">
        <Route path="/" component={Dashboard} />
        <Route path="/dashboard" component={Dashboard} />
        <Route path="/kanban" component={Kanban} />
        <Route path="/table" component={TableView} />
        <Route path="/categories" component={Categories} />
        <Route path="/profile" component={Profile} />
        <Route path="/skills" component={Skills} />
        <Route path="/artifacts" component={Artifacts} />
        <Route path="/settings" component={Settings} />
        <Route path="/job/:id" component={JobDetail} />
        <Route path="/artifact/:id" component={ArtifactDetail} />
      </Router>
    </div>
  </main>
</div>
