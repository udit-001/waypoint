<script>
import { setPage } from '../stores/page.svelte.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';

  import { skillLabel } from '../stores/skillMeta.js';

  let artifactsList = $state([]);
  let jobMap = $state({});

  onMount(async () => {
    setPage({ title: 'Artifacts' });

    await Promise.all([api.artifacts.ensure(), api.jobs.ensure()]);
    artifactsList = api.artifacts.value || [];

    // Build job ID → name lookup
    const jobs = api.jobs.value || [];
    const map = {};
    jobs.forEach(j => { map[j.id] = j.company; });
    jobMap = map;
  });

  function formatDate(d) {
    if (!d) return '';
    try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
    catch { return d; }
  }
</script>

{#if api.artifacts.loading && artifactsList.length === 0}
  <Spinner text="Loading artifacts..." />
{:else}
  <div class="space-y-4">

  {#if artifactsList.length === 0}
    <p class="text-sm text-slate-400">No artifacts generated yet.</p>
  {:else}
    {#each artifactsList as art}
      <button
        class="w-full text-left bg-white rounded-xl border border-slate-200 p-4 cursor-pointer hover:border-slate-400 hover:shadow-sm transition-all"
        onclick={() => router.navigate('/artifact/' + art.id)}
      >
        <div class="flex items-start justify-between mb-2">
          <span class="text-sm font-semibold text-slate-800">{art.title || 'Untitled'}</span>
          <span class="bg-slate-700 text-white rounded-full px-2 py-0.5 text-[10px] font-medium">{skillLabel(art.skillId)}</span>
        </div>
        <div class="text-xs text-slate-400 space-x-1">
          {#if art.jobId && jobMap[art.jobId]}
            <button
              class="text-slate-500 hover:text-slate-700 cursor-pointer bg-transparent border-none p-0 text-xs"
              onclick={(e) => { e.stopPropagation(); router.navigate('/job/' + art.jobId); }}
            >{jobMap[art.jobId]}</button>
            <span>·</span>
          {/if}
          {art.variants?.length || 0} variant{(art.variants?.length || 0) === 1 ? '' : 's'}
          · {formatDate(art.createdAt)}
        </div>
      </button>
    {/each}
  {/if}
</div>
{/if}
