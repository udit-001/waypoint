<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';

  let cats = $state([]);
  let jobCounts = $state({});

  onMount(async () => {
    setPage({ title: 'Categories' });

    await Promise.all([api.categories.ensure(), api.jobs.ensure()]);
    cats = api.categories.value || [];

    // Count jobs per category
    const counts = {};
    (api.jobs.value || []).forEach(j => {
      const cat = j.category || 'General';
      counts[cat] = (counts[cat] || 0) + 1;
    });
    jobCounts = counts;
  });

  function goToTable(category) {
    // TODO: pass category filter to table view via store/query param
    router.navigate('/table');
  }

  async function copyCmd(cmd) {
    await navigator.clipboard.writeText(cmd);
    // Brief feedback — could use a toast, keeping it simple
  }
</script>

{#if api.categories.loading && cats.length === 0}
  <Spinner text="Loading categories..." />
{:else}
  <div class="max-w-3xl mx-auto space-y-4">
  <p class="text-sm text-slate-400 mb-4">
    Organize your applications into categories. Manage them via the CLI.
  </p>

  <!-- All Categories -->
  <div>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
      {@html iconSvg("box", 14)} All Categories
    </h3>

    {#if cats.length === 0}
      <p class="text-sm text-slate-400">No categories yet.</p>
    {:else}
      <div class="bg-white rounded-xl border border-slate-200 overflow-hidden">
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
                      class="bg-slate-50 border border-slate-200 text-[11px] font-mono rounded px-2 py-0.5 cursor-pointer hover:border-slate-400 transition-colors"
                      onclick={() => copyCmd(`waypoint categories rename ${cat.id} "New Name"`)}
                      title="Copy rename command"
                    >
                      rename
                    </button>
                    {#if cat.id !== 1}
                      <button
                        class="bg-slate-50 border border-slate-200 text-[11px] font-mono rounded px-2 py-0.5 cursor-pointer hover:border-red-400 transition-colors"
                        onclick={() => copyCmd(`waypoint categories delete ${cat.id}`)}
                        title="Copy delete command"
                      >
                        delete
                      </button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>
{/if}
