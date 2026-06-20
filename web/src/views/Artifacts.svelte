<script>
  import { onMount } from 'svelte';
  import { navigate } from 'svelte-routing';
  import * as api from '../stores/api.js';

  const skillLabels = {
    'email-generator': 'Email',
    'cover-letter': 'Cover Letter',
    'resume-optimizer': 'Resume Optimizer',
    'interview-prep': 'Interview Prep',
    'career-summary': 'Career Summary',
    'statement-of-purpose': 'SOP',
  };

  let artifactsList = $state([]);

  onMount(async () => {
    await api.artifacts.ensure();
    artifactsList = api.artifacts.value || [];
  });

  function formatDate(d) {
    if (!d) return '';
    try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
    catch { return d; }
  }
</script>

<div class="max-w-3xl mx-auto space-y-4">
  <h2 class="text-xl font-semibold text-slate-800 mb-2">Artifacts</h2>

  {#if artifactsList.length === 0}
    <p class="text-sm text-slate-400">No artifacts generated yet.</p>
  {:else}
    {#each artifactsList as art}
      <button
        class="w-full text-left bg-white rounded-xl border border-slate-200 p-4 cursor-pointer hover:border-slate-400 hover:shadow-sm transition-all"
        onclick={() => navigate('/artifact/' + art.id)}
      >
        <div class="flex items-start justify-between mb-2">
          <span class="text-sm font-semibold text-slate-800">{art.title || 'Untitled'}</span>
          <span class="bg-slate-700 text-white rounded-full px-2 py-0.5 text-[10px] font-medium">{skillLabels[art.skillId] || art.skillId}</span>
        </div>
        <div class="text-xs text-slate-400">
          {art.variants?.length || 0} variant{(art.variants?.length || 0) === 1 ? '' : 's'}
          · {formatDate(art.createdAt)}
        </div>
      </button>
    {/each}
  {/if}
</div>
