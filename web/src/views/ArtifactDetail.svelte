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
  import { iconSvg } from '../lib/icons.js';

  let { id } = $props();

  let art = $state(null);
  let jobName = $state(null);
  let activeVariant = $state(0);
  let loading = $state(true);
  let copied = $state(false);
  let copiedId = $state(false);

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

  async function copyId() {
    await navigator.clipboard.writeText(`Waypoint artifact ${art.id}`);
    copiedId = true;
    setTimeout(() => copiedId = false, 1500);
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
      >{#if copied}<span class="inline-flex items-center gap-1">{@html iconSvg('check', 12)}Copied</span>{:else}Copy{/if}</button>
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

    <!-- Hand-off: a reference your assistant can resolve -->
    <div class="mt-6 flex items-center gap-3 flex-wrap">
      <span class="text-xs text-slate-400 dark:text-slate-500">Update this artifact through your assistant — it knows:</span>
      <button
        class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md border border-slate-200 dark:border-slate-600 bg-slate-50 dark:bg-slate-700 font-mono text-xs text-slate-700 dark:text-slate-200 hover:border-slate-400 cursor-pointer transition-colors"
        onclick={copyId}
        title="Copies 'Waypoint artifact {art.id}' — paste it into your assistant chat"
      >
        ARTIFACT&nbsp;{art.id}
        {#if copiedId}<span class="text-emerald-600 dark:text-emerald-400 inline-flex items-center gap-0.5 font-sans">{@html iconSvg('check', 11)}Copied</span>{/if}
      </button>
    </div>
  </div>
{/if}
