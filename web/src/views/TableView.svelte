<script>
import { setPage } from '../stores/page.svelte.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
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

  // Re-apply filters when the shared filter changes
  $effect(() => {
    if (filter.category !== undefined && filter.status !== undefined) applyFilters();
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
      class="bg-slate-50 border border-slate-200 rounded-lg px-3 py-1.5 text-sm w-48 focus:border-slate-400 focus:outline-none"
      oninput={applyFilters}
    />
  </div>

  <!-- Table -->
  <div class="bg-white rounded-xl border border-slate-200 overflow-x-auto">
    <table class="w-full text-sm whitespace-nowrap">
      <thead>
        <tr class="border-b border-slate-200">
          {#each columns as col}
            <th
              class="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-slate-400 cursor-pointer select-none hover:text-slate-600 sticky top-0 bg-white"
              onclick={() => sortBy(col.field)}
            >
              {col.label}
              {#if sortField === col.field}
                <span class="ml-1">{sortDir === 1 ? '▲' : '▼'}</span>
              {/if}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#if filteredJobs.length === 0}
          <tr>
            <td colspan={columns.length} class="text-center py-12 text-slate-400">No jobs found</td>
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
  </div>
</div>
{/if}
