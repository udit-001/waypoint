<script>
import { setPage } from '../stores/page.svelte.js';
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';

  import { skillLabel } from '../stores/skillMeta.js';
  import { formatDate } from '../lib/format.js';

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
    firstRender = false;
  });

  let firstRender = true;
  function stagger(i, opts) {
    const { y = 4, duration = 200, step = 30, cap = 8 } = opts || {};
    if (!firstRender) return { duration: 0 };
    return { y, duration, delay: Math.min(i, cap) * step };
  }

</script>

{#if api.artifacts.loading && artifactsList.length === 0}
  <Spinner text="Loading artifacts..." />
{:else}
  <div class="space-y-4">

  {#if artifactsList.length === 0}
    <div class="text-center py-12">
      <svg class="mx-auto text-slate-300 mb-3" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z"/><path d="M14 2v5a1 1 0 0 0 1 1h5"/></svg>
      <p class="text-sm text-slate-400 mb-1">No artifacts generated yet</p>
      <p class="text-xs text-slate-400">Run <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint skills run</code> to generate content</p>
    </div>
  {:else}
    {#each artifactsList as art, i (art.id)}
      <div in:fly={stagger(i, { y: 4, duration: 200, step: 30, cap: 8 })}>
        <Card
          onclick={() => router.navigate('/artifact/' + art.id)}
          padding="p-4"
          class="w-full text-left"
        >
        <div class="flex items-start justify-between mb-2">
          <span class="text-sm font-semibold text-slate-800">{art.title || 'Untitled'}</span>
          <span class="bg-slate-700 text-white rounded-full px-2 py-0.5 text-[10px] font-medium">{skillLabel(art.skillId)}</span>
        </div>
        <div class="text-xs text-slate-400 space-x-1">
          {#if art.jobId && jobMap[art.jobId]}
            <span
              class="text-slate-500 hover:text-slate-700 cursor-pointer bg-transparent border-none p-0 text-xs"
              onclick={(e) => { e.stopPropagation(); router.navigate('/job/' + art.jobId); }}
            >{jobMap[art.jobId]}</span>
            <span>·</span>
          {/if}
          {art.variants?.length || 0} variant{(art.variants?.length || 0) === 1 ? '' : 's'}
          · {formatDate(art.createdAt)}
        </div>
      </Card>
      </div>
    {/each}
  {/if}
</div>
{/if}
