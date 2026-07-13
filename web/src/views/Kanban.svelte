<script>
import { setPage } from '../stores/page.svelte.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { deadlineDaysLeft, deadlineLabel, deadlineClassMuted } from '../lib/deadline.js';
  import { formatDateShort } from '../lib/format.js';
  import { STATUSES, STATUS_STYLES } from '../lib/status.js';
  import { applyFilter } from '../lib/filter.js';

  const filter = getFilter();

  let allJobs = $state([]);

  onMount(async () => {
    setPage({ title: 'Kanban' });
    filter.sync();

    await api.jobs.ensure();
    allJobs = api.jobs.value || [];
  });

  function getJobsByStatus(statusFilter) {
    return applyFilter(allJobs, filter).filter(j => j.status === statusFilter);
  }

  function showJob(id) {
    router.navigate('/job/' + id);
  }
</script>

{#if api.jobs.loading && allJobs.length === 0}
  <Spinner text="Loading jobs..." />
{:else}
  <div class="flex gap-4 min-h-[calc(100vh-12rem)] pb-4 overflow-x-auto">
  {#each STATUSES as status}
    <div class="flex flex-col flex-1 min-w-[280px] max-w-[320px] bg-slate-50/50 rounded-xl border-t-2 {STATUS_STYLES[status]?.border} p-3">
      <div class="flex items-center justify-between px-2 pb-3">
        <span class="text-xs font-semibold uppercase tracking-wide text-slate-500">{status}</span>
        <span class="bg-slate-200/80 text-slate-600 rounded-full px-2 py-0.5 text-xs font-medium tabular-nums">
          {getJobsByStatus(status).length}
        </span>
      </div>
      <div class="flex flex-col gap-2 flex-1 min-h-[60px] overflow-y-auto">
        {#each getJobsByStatus(status) as job}
          {@const days = deadlineDaysLeft(job.date)}
          <button
            class="bg-white rounded-lg border border-slate-200 p-3 text-left cursor-pointer hover:border-slate-400 hover:shadow-sm hover:-translate-y-0.5 transition-all"
            onclick={() => showJob(job.id)}
          >
            <div class="text-sm font-semibold text-slate-800 mb-0.5">{job.company}</div>
            <div class="text-xs text-slate-500">{job.position}</div>
            <div class="flex flex-wrap gap-1.5 mt-2 text-xs text-slate-400">
              {#if days !== null}
                <span class="font-medium {deadlineClassMuted(days)}">{deadlineLabel(days)}</span>
              {/if}
              {#if job.salary}<span>{job.salary}</span>{/if}
              {#if job.location}<span>{job.location}</span>{/if}
              {#if job.appliedDate}<span>Applied: {formatDateShort(job.appliedDate)}</span>{/if}
            </div>
            <div class="mt-2">
              <span class="bg-slate-100 text-slate-500 rounded px-1.5 py-0.5 text-[10px] uppercase font-semibold">{job.category || 'Uncategorized'}</span>
            </div>
          </button>
        {/each}
      </div>
    </div>
  {/each}
</div>
{/if}
