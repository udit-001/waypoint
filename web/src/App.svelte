<script>
  import IconRail from './components/IconRail.svelte';
  import TopBar from './components/TopBar.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Applications from './views/Applications.svelte';
  import GetStarted from './views/GetStarted.svelte';
  import FoundJobs from './views/FoundJobs.svelte';
  import Companies from './views/Companies.svelte';
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
  import { setup } from './lib/onboarding.svelte.js';
  import { briefStatus } from './lib/brief.js';
  import * as api from './stores/api.svelte.js';

  const router = getRouter();

  // First-run redirect: if onboarding hasn't been dismissed, check whether
  // setup is actually complete (profile, preferences, keys, autopilot). If
  // any piece is missing, send the user to the wizard.
  $effect(() => {
    if (!setup.dismissed && router.current.route === 'applications') {
      const check = async () => {
        try {
          const [profile, brief, autopilot] = await Promise.all([
            api.profile.ensure().then(() => api.profile.value),
            api.brief.ensure().then(() => api.brief.value),
            api.autopilot.ensure().then(() => api.autopilot.value),
          ]);

          const profileDone = !!(profile?.name || (profile?.skills?.length ?? 0) > 0 || (profile?.experience?.length ?? 0) > 0);
          const prefsDone = briefStatus(brief).complete;
          const zenDone = !!autopilot?.zenKeySet;
          const exaDone = !!autopilot?.exaKeySet;
          const autopilotDone = !!autopilot?.enabled;

          if (!(profileDone && prefsDone && zenDone && exaDone && autopilotDone)) {
            router.replace('/get-started');
          }
        } catch { /* server down — stay on applications */ }
      };
      check();
    }
  });

  // Set correct page title immediately — before any view mounts
  const routeTitles = {
    applications: 'Applications', found: 'Matches', companies: 'Companies', categories: 'Categories',
    profile: 'Profile', skills: 'AI Skills', artifacts: 'Artifacts',
    settings: 'Settings', job: 'Job Detail', artifact: 'Artifact',
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
      {#if router.current.route === 'get-started'}
        <GetStarted />
      {:else if router.current.route === 'applications'}
        <Applications />
      {:else if router.current.route === 'found'}
        <FoundJobs />
      {:else if router.current.route === 'companies'}
        <Companies />
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
