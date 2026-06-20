<script>
  import { onMount } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';
  import { setPage } from '../stores/page.svelte.js';
  import { skillLabel } from '../stores/skillMeta.js';
  import { marked } from 'marked';
  import Spinner from '../components/Spinner.svelte';

  function renderMarkdown(text) {
    if (!text) return '';
    try {
      return marked.parse(text, { gfm: true, breaks: true });
    } catch {
      return text;
    }
  }

  let { id } = $props();

  let job = $state(null);
  let history = $state([]);
  let linkedArtifacts = $state([]);
  let loading = $state(true);

  onMount(async () => {
    loading = true;
    job = await api.getJob(parseInt(id));
    if (!job) { router.navigate('/'); return; }
    history = await api.getJobHistory(job.id);
    loading = false;

    // Filter artifacts linked to this job
    await api.artifacts.ensure();
    linkedArtifacts = (api.artifacts.value || []).filter(a => a.jobId === job.id).slice(0, 10);
    setPage({
      title: `${job.company} — ${job.position}`,
      breadcrumbs: [
        { label: 'Jobs', action: () => router.navigate('/table') },
        { label: `${job.company} — ${job.position}` },
      ],
    });
  });

  function formatDate(d) {
    if (!d) return '-';
    try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
    catch { return d; }
  }

  function formatDateTime(d) {
    if (!d) return '-';
    try { return new Date(d).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }); }
    catch { return d; }
  }
</script>

{#if loading}
  <Spinner text="Loading job..." />
{:else if job}
  <div class="max-w-3xl mx-auto">
    <!-- Header -->
    <div class="flex justify-between items-start gap-4 mb-6 border-b border-slate-200 pb-4">
      <div>
        <h2 class="text-xl font-bold text-slate-800">{job.company}</h2>
        <p class="text-sm text-slate-500 mt-0.5">{job.position}</p>
      </div>
      <span class="bg-blue-100 text-blue-700 rounded px-2.5 py-1 text-xs font-medium">{job.status}</span>
    </div>

    <!-- Grid -->
    <div class="grid grid-cols-2 gap-x-8 gap-y-3 mb-6">
      <div><span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">Category</span><span class="text-sm text-slate-700">{job.category || 'General'}</span></div>
      <div><span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">Salary</span><span class="text-sm text-slate-700">{job.salary || '-'}</span></div>
      <div><span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">Location</span><span class="text-sm text-slate-700">{job.location || '-'}</span></div>
      <div><span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">Contact</span><span class="text-sm text-slate-700">{job.contact || '-'}</span></div>
      <div><span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">Deadline</span><span class="text-sm text-slate-700">{formatDate(job.date)}</span></div>
      <div><span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">Applied</span><span class="text-sm text-slate-700">{formatDate(job.appliedDate)}</span></div>
    </div>

    {#if job.url}
      <div class="mb-6">
        <span class="block text-[11px] uppercase tracking-wide text-slate-400 font-semibold">URL</span>
        <a href={job.url} target="_blank" class="text-sm text-slate-600 hover:text-slate-500 break-all">{job.url}</a>
      </div>
    {/if}

    {#if job.notes}
      <div class="mb-6">
        <h4 class="text-sm font-semibold text-slate-700 border-b border-slate-200 pb-2 mb-3">Notes</h4>
        <div class="bg-slate-50 rounded-lg p-4 text-sm text-slate-700 leading-relaxed notes-content">{@html renderMarkdown(job.notes)}</div>
      </div>
    {/if}

    {#if linkedArtifacts.length > 0}
      <div class="mb-6">
        <h4 class="text-sm font-semibold text-slate-700 border-b border-slate-200 pb-2 mb-3">Linked Artifacts</h4>
        <div class="space-y-2">
          {#each linkedArtifacts as a}
            <button
              class="w-full text-left bg-white rounded-lg border border-slate-200 p-3 cursor-pointer hover:border-slate-400 transition-all"
              onclick={() => router.navigate('/artifact/' + a.id)}
            >
              <div class="flex items-center gap-2 text-sm">
                <span class="font-medium text-slate-800">{a.title || 'Untitled'}</span>
                <span class="bg-slate-600 text-white rounded-full px-2 py-0.5 text-[10px]">{skillLabel(a.skillId)}</span>
              </div>
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <!-- History -->
    <div class="mb-6">
      <h4 class="text-sm font-semibold text-slate-700 border-b border-slate-200 pb-2 mb-3">Activity History</h4>
      {#if history.length === 0}
        <p class="text-xs text-slate-400">No history recorded yet.</p>
      {:else}
        <div class="space-y-1.5">
          {#each history as h}
            <div class="flex gap-3 text-xs">
              <span class="text-slate-400 shrink-0 tabular-nums">{formatDateTime(h.timestamp)}</span>
              <span class="text-slate-600">
                {#if h.action === 'Created'}Job created
                {:else if h.action === 'Status'}{h.from} → <span class="font-medium">{h.to}</span>
                {:else}{h.action}
                {/if}
              </span>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <!-- CLI -->
    <div class="mb-6">
      <h4 class="text-sm font-semibold text-slate-700 border-b border-slate-200 pb-2 mb-3">CLI Quick Actions</h4>
      <pre class="bg-slate-50 p-4 rounded-lg text-sm text-slate-600 leading-relaxed overflow-x-auto font-mono">  waypoint jobs update {job.id} --status "Offer" --notes "New status"
  waypoint jobs update {job.id} --notes "Add a note here"
  waypoint jobs delete {job.id}</pre>
    </div>
  </div>
{/if}

<style>
  .notes-content h1, .notes-content h2, .notes-content h3, .notes-content h4 {
    font-weight: 600;
    line-height: 1.3;
    margin: 16px 0 8px;
  }
  .notes-content h1 { font-size: 1.25rem; }
  .notes-content h2 { font-size: 1.1rem; border-bottom: 1px solid var(--color-slate-200); padding-bottom: 4px; }
  .notes-content h3 { font-size: 1rem; }
  .notes-content h4 { font-size: 0.925rem; }
  .notes-content h1:first-child, .notes-content h2:first-child, .notes-content h3:first-child { margin-top: 0; }
  .notes-content p { margin: 0 0 8px; }
  .notes-content p:last-child { margin: 0; }
  .notes-content ul, .notes-content ol { margin: 0 0 8px; padding-left: 20px; }
  .notes-content li { margin-bottom: 4px; }
  .notes-content li > ul, .notes-content li > ol { margin-bottom: 0; }
  .notes-content blockquote {
    border-left: 3px solid var(--color-slate-600);
    background: var(--color-slate-100);
    border-radius: 0 4px 4px 0;
    margin: 8px 0;
    padding: 8px 12px;
    font-style: italic;
  }
  .notes-content table { border-collapse: collapse; width: 100%; margin: 8px 0; font-size: 0.875rem; }
  .notes-content th, .notes-content td { text-align: left; border-bottom: 1px solid var(--color-slate-200); padding: 6px 10px; }
  .notes-content th {
    color: var(--color-slate-600);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .notes-content code {
    background: var(--color-slate-100);
    border-radius: 3px;
    padding: 2px 5px;
    font-size: 0.8rem;
  }
  .notes-content pre {
    background: var(--color-slate-100);
    border-radius: 6px;
    margin: 8px 0;
    padding: 12px;
    font-size: 0.8rem;
    overflow-x: auto;
  }
  .notes-content pre code { background: none; padding: 0; }
  .notes-content a { color: var(--color-slate-600); text-decoration: none; }
  .notes-content a:hover { text-decoration: underline; }
  .notes-content hr { border: none; border-top: 1px solid var(--color-slate-200); margin: 12px 0; }
  .notes-content input[type="checkbox"] { accent-color: var(--color-slate-600); margin-right: 6px; }
</style>
