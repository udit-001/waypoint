<script>
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';

  onMount(() => { setPage({ title: 'Search' }); });
  const skillLabels = {
    'email-generator': 'Email',
    'cover-letter': 'Cover Letter',
    'resume-optimizer': 'Resume Optimizer',
    'interview-prep': 'Interview Prep',
    'career-summary': 'Career Summary',
    'statement-of-purpose': 'SOP',
  };

  let query = $state('');
  let results = $state([]);
  let loading = $state(false);
  let searched = $state(false);

  async function doSearch() {
    if (query.length < 2) return;
    loading = true;
    searched = true;
    try {
      results = await api.searchAll(query);
    } catch {
      results = [];
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Enter') doSearch();
  }

  function highlight(text, q) {
    if (!q) return text;
    const escaped = q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    return text.replace(new RegExp(`(${escaped})`, 'gi'), '<mark class="bg-yellow-200 text-inherit rounded px-0.5">$1</mark>');
  }

  function openResult(type, id) {
    router.navigate(type === 'job' ? `/job/${id}` : `/artifact/${id}`);
  }
</script>

<div class="max-w-3xl">

  <div class="flex gap-2 mb-6">
    <input
      type="text"
      bind:value={query}
      placeholder="Search jobs & artifacts..."
      class="flex-1 bg-white border border-slate-200 rounded-lg px-4 py-2.5 text-sm focus:border-slate-400 focus:outline-none"
      onkeydown={handleKeydown}
    />
    <button
      class="px-5 py-2.5 bg-slate-800 text-white rounded-lg text-sm font-medium cursor-pointer hover:bg-slate-700 transition-colors"
      onclick={doSearch}
      disabled={query.length < 2}
    >
      Search
    </button>
  </div>

  {#if loading}
    <p class="text-sm text-slate-400">Searching...</p>
  {:else if searched}
    {#if results.length === 0}
      <p class="text-sm text-slate-400">No results for "{query}"</p>
    {:else}
      <p class="text-sm text-slate-400 mb-4">{results.length} result{results.length === 1 ? '' : 's'}</p>

      <div class="space-y-2">
        {#each results as result}
          <button
            class="w-full text-left bg-white rounded-lg border border-slate-200 p-4 cursor-pointer hover:border-slate-400 hover:shadow-sm transition-all"
            onclick={() => openResult(result.type, result.id)}
          >
            <div class="flex items-center gap-3">
              {@html iconSvg(result.type === 'job' ? 'briefcase' : 'file-text', 18)}
              <div class="flex-1 min-w-0">
                <div class="text-sm font-medium text-slate-800 truncate">
                  {@html highlight(result.title || 'Untitled', query)}
                </div>
                <div class="text-xs text-slate-400 mt-0.5">
                  {result.type === 'job' ? 'Job' : (skillLabels[result.sub] || result.sub || 'Artifact')}
                </div>
              </div>
            </div>
          </button>
        {/each}
      </div>
    {/if}
  {:else}
    <p class="text-sm text-slate-400">Type at least 2 characters and press Enter or Search.</p>
  {/if}
</div>
