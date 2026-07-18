<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';

  let cats = $state([]);
  let jobCounts = $state({});
  let copiedCmd = $state('');

  onMount(async () => {
    setPage({ title: 'Categories' });

    await Promise.all([api.categories.ensure(), api.jobs.ensure()]);
    cats = api.categories.value || [];

    // Count jobs per category
    const counts = {};
    (api.jobs.value || []).forEach(j => {
      const cat = j.category || 'Uncategorized';
      counts[cat] = (counts[cat] || 0) + 1;
    });
    jobCounts = counts;
  });

  function goToTable(category) {
    router.navigate('/applications?category=' + encodeURIComponent(category));
  }

  async function copyCmd(cmd, key) {
    await navigator.clipboard.writeText(cmd);
    copiedCmd = key;
    setTimeout(() => { if (copiedCmd === key) copiedCmd = ''; }, 1500);
  }
</script>

{#if api.categories.loading && cats.length === 0}
  <Spinner text="Loading categories..." />
{:else}
  <div class="space-y-4">
  <p class="text-sm text-slate-400 mb-4">
    Organize your applications into categories. Manage them via the CLI.
  </p>

  <!-- All Categories -->
  <div>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
      {@html iconSvg("box", 14)} All Categories
    </h3>

    {#if cats.length === 0}
      <div class="text-center py-12">
        <svg class="mx-auto text-slate-300 mb-3" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg>
        <p class="text-sm text-slate-400 mb-1">No categories yet</p>
        <p class="text-xs text-slate-400">Run <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint categories add</code> to create one</p>
      </div>
    {:else}
      <Card hover={false} padding="p-0" class="overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200">
              <th class="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-400">Name</th>
              <th class="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-400">Jobs</th>
              <th class="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-400">Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each cats as cat}
              <tr class="border-b border-slate-100 hover:bg-slate-50 transition-colors">
                <td class="px-4 py-2.5">
                  <button
                    class="text-slate-700 font-medium hover:text-slate-500 cursor-pointer bg-transparent border-none p-0 text-sm"
                    onclick={() => goToTable(cat.name)}
                  >
                    {cat.name}
                  </button>
                  {#if cat.id === 1}
                    <span class="ml-1.5 bg-slate-700 text-white text-[10px] font-medium rounded-full px-1.5 py-0.5 align-middle">default</span>
                  {/if}
                </td>
                <td class="px-4 py-2.5">
                  <button
                    class="text-slate-700 hover:text-slate-500 cursor-pointer bg-transparent border-none p-0 text-sm tabular-nums"
                    onclick={() => goToTable(cat.name)}
                  >
                    {jobCounts[cat.name] || 0}
                  </button>
                </td>
                <td class="px-4 py-2.5">
                  <div class="flex gap-1.5">
                    <button
                      class="bg-slate-50 border border-slate-200 text-[11px] font-mono rounded px-3 py-1.5 cursor-pointer hover:border-slate-400 transition-colors {copiedCmd === 'rename-' + cat.id ? 'text-emerald-700 border-emerald-400' : ''}"
                      onclick={() => copyCmd(`waypoint categories rename ${cat.id} "New Name"`, 'rename-' + cat.id)}
                      title="Copy rename command"
                    >
                      {copiedCmd === 'rename-' + cat.id ? '✓ Copied' : 'rename'}
                    </button>
                    {#if cat.id !== 1}
                      <button
                        class="bg-slate-50 border border-slate-200 text-[11px] font-mono rounded px-3 py-1.5 cursor-pointer hover:border-red-400 transition-colors {copiedCmd === 'delete-' + cat.id ? 'text-emerald-700 border-emerald-400' : ''}"
                        onclick={() => copyCmd(`waypoint categories delete ${cat.id}`, 'delete-' + cat.id)}
                        title="Copy delete command"
                      >
                        {copiedCmd === 'delete-' + cat.id ? '✓ Copied' : 'delete'}
                      </button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </Card>
    {/if}
  </div>
</div>
{/if}
