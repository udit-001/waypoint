<script>
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';
  import { setPage } from '../stores/page.svelte.js';
  import { skillLabel } from '../stores/skillMeta.js';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import { formatDateFull } from '../lib/format.js';

  let { id } = $props();

  let art = $state(null);
  let jobName = $state(null);
  let activeVariant = $state(0);
  let loading = $state(true);
  let copied = $state(false);
  let cliPre = $state(null);
  let copiedCli = $state(false);

  onMount(async () => {
    loading = true;
    art = await api.getArtifact(parseInt(id));
    if (!art) { router.navigate('/artifacts'); return; }
    loading = false;

    // Build breadcrumbs with optional job link
    const crumbs = [
      { label: 'Artifacts', action: () => router.navigate('/artifacts') },
    ];
    if (art.jobId) {
      const job = await api.getJob(art.jobId);
      if (job) {
        jobName = job.company;
        crumbs.push({ label: job.company, action: () => router.navigate('/job/' + job.id) });
      }
    }
    crumbs.push({ label: art.title || 'Artifact' });

    setPage({
      title: art.title || 'Artifact',
      breadcrumbs: crumbs,
    });
  });

  async function copyContent() {
    if (!art?.variants?.[activeVariant]) return;
    await navigator.clipboard.writeText(art.variants[activeVariant].content || '');
    copied = true;
    setTimeout(() => copied = false, 1500);
  }

  async function copyCli() {
    if (!cliPre) return;
    await navigator.clipboard.writeText(cliPre.textContent);
    copiedCli = true;
    setTimeout(() => copiedCli = false, 1500);
  }
</script>

{#if loading}
  <Spinner text="Loading artifact..." />
{:else if art}
  <div class="max-w-3xl">
    <div class="mb-6">
      <h2 class="text-xl font-bold text-slate-800">{art.title || 'Untitled'}</h2>
      <div class="flex items-center gap-2 mt-1 text-xs text-slate-400 flex-wrap">
        <span class="bg-slate-700 text-white rounded-full px-2 py-0.5 text-[10px] font-medium">{skillLabel(art.skillId)}</span>
        {#if jobName}
          <button
            class="text-slate-500 hover:text-slate-700 cursor-pointer bg-transparent border-none p-0 text-xs"
            onclick={() => router.navigate('/job/' + art.jobId)}
          >{jobName}</button>
          <span>·</span>
        {/if}
        <span>{formatDateFull(art.createdAt)}</span>
      </div>
    </div>

    <!-- Content header -->
    <div class="flex items-center gap-2 mb-2">
      <h4 class="text-sm font-semibold text-slate-700">Content</h4>
      <button
        class="px-2.5 py-1 rounded text-xs font-medium cursor-pointer transition-colors {copied ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'}"
        onclick={copyContent}
      >{copied ? '✓ Copied' : 'Copy'}</button>
    </div>

    <!-- Variant tabs -->
    {#if art.variants?.length > 1}
      <div class="flex border-b border-slate-200 gap-1 mb-0">
        {#each art.variants as v, i}
          <button
            class="px-4 py-2 text-sm cursor-pointer border-b-2 transition-colors {activeVariant === i ? 'border-slate-700 text-slate-700 font-medium' : 'border-transparent text-slate-400 hover:text-slate-600'}"
            onclick={() => activeVariant = i}
          >{v.label || 'Variant ' + (i + 1)}</button>
        {/each}
      </div>
    {/if}

    <!-- Variant content -->
    {#if art.variants?.[activeVariant]}
      <Card hover={false} padding="p-6" class="mt-2 text-sm text-slate-700 leading-relaxed whitespace-pre-wrap break-words max-h-[400px] overflow-y-auto">
        {art.variants[activeVariant].content || ''}
      </Card>
    {/if}

    <!-- CLI -->
    <div class="mt-6">
      <h4 class="text-sm font-semibold text-slate-700 border-b border-slate-200 pb-2 mb-3">CLI</h4>
      <div class="relative">
        <button
          class="absolute top-2 right-2 px-2.5 py-1 rounded text-xs font-medium cursor-pointer transition-colors {copiedCli ? 'bg-emerald-100 text-emerald-700' : 'bg-white text-slate-600 hover:bg-slate-100 border border-slate-200'}"
          onclick={copyCli}
        >{copiedCli ? '✓ Copied' : 'Copy'}</button>
        <pre bind:this={cliPre} class="bg-slate-50 p-4 rounded-lg text-sm text-slate-600 leading-relaxed overflow-x-auto font-mono">waypoint artifacts get {art.id}
waypoint artifacts archive {art.id}
waypoint artifacts delete {art.id}</pre>
      </div>
    </div>
  </div>
{/if}
