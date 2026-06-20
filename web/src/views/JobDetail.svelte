<script>
  import { onMount } from 'svelte';
  import { navigate } from 'svelte-routing';
  import * as api from '../stores/api.js';

  let { id } = $props();

  let job = $state(null);
  let history = $state([]);
  let loading = $state(true);

  onMount(async () => {
    loading = true;
    job = await api.getJob(parseInt(id));
    if (!job) { navigate('/'); return; }
    history = await api.getJobHistory(job.id);
    loading = false;
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
  <p class="text-sm text-slate-400">Loading...</p>
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
        <div class="bg-slate-50 rounded-lg p-4 text-sm text-slate-700 leading-relaxed whitespace-pre-wrap">{job.notes}</div>
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
