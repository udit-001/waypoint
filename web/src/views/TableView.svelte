<script>
import { setPage } from '../stores/page.svelte.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { deadlineDaysLeft, deadlineLabel, deadlineClass, deadlineRowTint } from '../lib/deadline.js';
  import { formatDateShort } from '../lib/format.js';
  import { STATUS_STYLES } from '../lib/status.js';
  import { applyFilter } from '../lib/filter.js';

  const filter = getFilter();

  const columns = [
    { field: 'company', label: 'Company' },
    { field: 'position', label: 'Position' },
    { field: 'date', label: 'Deadline' },
    { field: 'status', label: 'Status' },
    { field: 'category', label: 'Category' },
    { field: 'appliedDate', label: 'Applied' },
    { field: 'salary', label: 'Salary' },
    { field: 'location', label: 'Location' },
  ];

  let jobs = $state([]);
  let filteredJobs = $state([]);
  let sortField = $state('date');
  let sortDir = $state(1);
  let searchQuery = $state('');

  onMount(async () => {
    setPage({ title: 'Table View' });
    filter.sync();

    await api.jobs.ensure();
    jobs = api.jobs.value || [];
    applyFilters();
  });

  // Re-apply filters when the shared filter changes. The reads of
  // filter.categories/statuses/deadlineBucket/stale/textQuery inside
  // applyFilters are what make this $effect reactive.
  $effect(() => {
    applyFilters();
  });

  function sortBy(field) {
    if (sortField === field) sortDir *= -1;
    else { sortField = field; sortDir = 1; }
    applyFilters();
  }

  function applyFilters() {
    let result = applyFilter(jobs, filter);

    // Search filter
    if (searchQuery.length >= 2) {
      const q = searchQuery.toLowerCase();
      result = result.filter(j =>
        j.company.toLowerCase().includes(q) ||
        j.position.toLowerCase().includes(q) ||
        (j.notes && j.notes.toLowerCase().includes(q)) ||
        (j.location && j.location.toLowerCase().includes(q))
      );
    }

    // Sort
    result.sort((a, b) => {
      let va = (a[sortField] || '').toString().toLowerCase();
      let vb = (b[sortField] || '').toString().toLowerCase();
      return va < vb ? -sortDir : va > vb ? sortDir : 0;
    });

    filteredJobs = result;
  }

  function showJob(id) {
    router.navigate('/job/' + id);
  }
</script>

{#if api.jobs.loading && filteredJobs.length === 0}
  <Spinner text="Loading jobs..." />
{:else}
  <div class="space-y-4">
  <!-- Filters -->
  <div class="flex flex-wrap items-center gap-2">
    <input
      type="text"
      bind:value={searchQuery}
      placeholder="Search..."
      class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-1.5 text-sm w-48 focus:border-slate-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-400/40"
      oninput={applyFilters}
    />
  </div>

  <!-- Table -->
  <Card hover={false} padding="p-0" class="overflow-x-auto">
    <table class="w-full text-sm whitespace-nowrap">
      <thead>
        <tr class="border-b border-slate-200">
          {#each columns as col}
            <th
              class="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-400 cursor-pointer select-none hover:text-slate-600 transition-colors"
              onclick={() => sortBy(col.field)}
            >
              {col.label}
              {#if sortField === col.field}
                <svg class="inline-block ml-1 align-middle" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                  {#if sortDir === 1}<path d="m18 15-6-6-6 6"/>{:else}<path d="m6 9 6 6 6-6"/>{/if}
                </svg>
              {/if}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#if filteredJobs.length === 0}
          <tr>
            <td colspan={columns.length} class="text-center py-12">
              <svg class="mx-auto text-slate-300 mb-3" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="14" x="2" y="6" rx="2"/><path d="M16 20V4a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/></svg>
              <p class="text-sm text-slate-400 mb-1">No jobs found</p>
              <p class="text-xs text-slate-400">Run <code class="bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded font-mono text-[11px]">waypoint jobs add</code> to get started</p>
            </td>
          </tr>
        {:else}
          {#each filteredJobs as job}
            {@const days = deadlineDaysLeft(job.date)}
            <tr
              class="border-b border-slate-100 hover:bg-slate-50 cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-slate-400 {deadlineRowTint(days, job.status)}"
              onclick={() => showJob(job.id)}
              tabindex="0"
              onkeydown={(e) => { if (e.key === 'Enter') showJob(job.id); }}
            >
              <td class="px-4 py-2.5 font-medium text-slate-800">{job.company}</td>
              <td class="px-4 py-2.5 text-slate-600">{job.position}</td>
              <td class="px-4 py-2.5">
                <span class="tabular-nums text-slate-500">{formatDateShort(job.date)}</span>
                {#if days !== null}
                  <span class="ml-1.5 text-xs font-semibold {deadlineClass(days)}">{deadlineLabel(days)}</span>
                {/if}
              </td>
              <td class="px-4 py-2.5">
                <span class="inline-block px-2 py-0.5 rounded text-xs font-medium {STATUS_STYLES[job.status]?.bg || 'bg-slate-100 text-slate-600'}">{job.status}</span>
              </td>
              <td class="px-4 py-2.5 text-slate-600">{job.category || 'Uncategorized'}</td>
              <td class="px-4 py-2.5 text-slate-500 tabular-nums">{formatDateShort(job.appliedDate) || '-'}</td>
              <td class="px-4 py-2.5 text-slate-600">{job.salary || ''}</td>
              <td class="px-4 py-2.5 text-slate-500">{job.location || '-'}</td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </Card>
</div>
{/if}
