<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import Spinner from '../components/Spinner.svelte';
  import { onMount, onDestroy } from 'svelte';
  import { getRouter } from '../stores/router.svelte.js';
  const router = getRouter();
  import * as api from '../stores/api.svelte.js';
  import { getFilter } from '../stores/filter.svelte.js';
  import { Chart, registerables } from 'chart.js';

  const filter = getFilter();

  Chart.register(...registerables);

  let chartInstances = [];
  let pipelineCanvas = null;
  let salaryCanvas = null;

  const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

  let jobs = $state([]);
  let allHistory = $state([]);
  let loaded = $state(false);

  // Filtered jobs based on selected category and status
  let filteredJobs = $derived.by(() => {
    let result = jobs || [];
    if (filter.category) {
      result = result.filter(j => j.category === filter.category);
    }
    if (filter.status) {
      result = result.filter(j => j.status === filter.status);
    }
    return result;
  });

  let hasActiveFilter = $derived(filter.category || filter.status);

  // Derived state
  let statusCounts = $derived.by(() => {
    const counts = {};
    (filteredJobs || []).forEach(j => { counts[j.status] = (counts[j.status] || 0) + 1; });
    return counts;
  });

  let notApplied = $derived(statusCounts['Not Applied'] || 0);
  let applied = $derived(statusCounts['Applied'] || 0);
  let offers = $derived(statusCounts['Offer'] || 0);
  let rejected = $derived(statusCounts['Rejected'] || 0);
  let activePipeline = $derived(notApplied + applied);
  let responseRate = $derived(applied > 0 ? Math.round((offers + rejected) / applied * 100) : 0);
  let offerRate = $derived(applied > 0 ? Math.round(offers / applied * 100) : 0);

  // Pipeline chart data
  let pipelineData = $derived.by(() => {
    const stages = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];
    return stages.filter(s => statusCounts[s] > 0).map(s => ({ stage: s, count: statusCounts[s] }));
  });

  // Salary chart data
  let salaryData = $derived.by(() => {
    return (jobs || [])
      .filter(j => j.salary && j.salary.trim())
      .map(j => {
        const parsed = parseSalary(j.salary);
        return { ...j, ...parsed };
      })
      .filter(j => j.mid > 0)
      .sort((a, b) => b.mid - a.mid)
      .slice(0, 12);
  });

  // Week-over-week
  let thisWeek = $derived.by(() => {
    const weekAgo = new Date(Date.now() - 7 * 86400000);
    return (filteredJobs || []).filter(j => j.appliedDate && new Date(j.appliedDate) >= weekAgo).length;
  });
  let lastWeek = $derived.by(() => {
    const weekAgo = new Date(Date.now() - 7 * 86400000);
    const twoWeeksAgo = new Date(Date.now() - 14 * 86400000);
    return (filteredJobs || []).filter(j => j.appliedDate && new Date(j.appliedDate) >= twoWeeksAgo && new Date(j.appliedDate) < weekAgo).length;
  });
  let weekDelta = $derived(lastWeek > 0 ? Math.round((thisWeek - lastWeek) / lastWeek * 100) : (thisWeek > 0 ? 100 : 0));

  // Actions this week
  let actionsThisWeek = $derived.by(() => {
    const weekAgo = new Date(Date.now() - 7 * 86400000);
    return (allHistory || []).filter(h => new Date(h.timestamp) >= weekAgo).length;
  });

  // Stale applications
  let staleApps = $derived.by(() => {
    const now = new Date();
    return (jobs || []).filter(j => {
      if (j.status !== 'Applied' || !j.appliedDate) return false;
      return Math.round((now - new Date(j.appliedDate)) / 86400000) > 14;
    }).map(j => ({
      ...j,
      daysStale: Math.round((now - new Date(j.appliedDate)) / 86400000)
    })).sort((a, b) => b.daysStale - a.daysStale);
  });

  // Upcoming deadlines
  let upcoming = $derived.by(() => {
    const now = new Date();
    return (jobs || [])
      .filter(j => j.date && j.status !== 'Offer' && j.status !== 'Withdrawn' && j.status !== 'Rejected')
      .map(j => ({ ...j, daysLeft: Math.ceil((new Date(j.date) - now) / 86400000) }))
      .sort((a, b) => a.daysLeft - b.daysLeft);
  });

  // Category breakdown
  let catCounts = $derived.by(() => {
    const counts = {};
    (jobs || []).forEach(j => { const c = j.category || 'General'; counts[c] = (counts[c] || 0) + 1; });
    return Object.entries(counts).map(([label, value]) => ({ label, value })).sort((a, b) => b.value - a.value);
  });

  // Recent activity
  let recentActivity = $derived.by(() => {
    return (allHistory || []).filter(h => h.action === 'Status' && h.from).slice(0, 8);
  });

  onMount(async () => {
    setPage({ title: 'Dashboard' });

    await Promise.all([api.jobs.ensure(), api.history.ensure()]);
    jobs = api.jobs.value || [];
    allHistory = api.history.value || [];
    loaded = true;
  });

  onDestroy(() => {
    chartInstances.forEach(c => c.destroy());
    chartInstances = [];
  });

  // Render charts after DOM updates
  $effect(() => {
    if (loaded && jobs.length > 0) {
      // Wait for DOM to render
      requestAnimationFrame(() => {
        destroyCharts();
        renderPipelineChart();
        renderSalaryChart();
      });
    }
  });

  function destroyCharts() {
    chartInstances.forEach(c => c.destroy());
    chartInstances = [];
  }

  function renderPipelineChart() {
    const canvas = document.getElementById('chart-pipeline');
    if (!canvas || pipelineData.length === 0) return;

    const accentColor = getComputedStyle(document.documentElement).getPropertyValue('--color-slate-600').trim() || '#88c0d0';
    const mutedColor = getComputedStyle(document.documentElement).getPropertyValue('--color-slate-400').trim() || '#81a1c1';
    const textColor = getComputedStyle(document.documentElement).getPropertyValue('--color-slate-700').trim() || '#aebbcf';
    const gridColor = getComputedStyle(document.documentElement).getPropertyValue('--color-slate-200').trim() || '#434c5e';

    const ctx = canvas.getContext('2d');
    const labels = pipelineData.map(d => d.stage);
    const values = pipelineData.map(d => d.count);
    const appliedCount = pipelineData.find(d => d.stage === 'Applied')?.count || 0;

    const tickLabels = labels.map((s, i) => {
      const count = values[i];
      if ((s === 'Offer' || s === 'Rejected') && appliedCount > 0) {
        return `${s}  (${Math.round(count / appliedCount * 100)}%)`;
      }
      return s;
    });

    const colors = labels.map(s => s === 'Offer' ? '#bf616a' : '#4c566a');

    const chart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: tickLabels,
        datasets: [{
          data: values,
          backgroundColor: colors,
          barThickness: 20,
          borderRadius: 2,
        }]
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: '#2e3440',
            titleColor: '#eceff4',
            bodyColor: '#d8dee9',
            padding: 8,
            cornerRadius: 4,
            displayColors: false,
          },
        },
        scales: {
          x: {
            beginAtZero: true,
            grid: { color: gridColor, drawTicks: false },
            border: { color: gridColor },
            ticks: { color: textColor, stepSize: 1, padding: 4 },
          },
          y: {
            grid: { display: false },
            border: { display: false },
            ticks: { color: textColor, padding: 4 },
          },
        },
      }
    });
    chartInstances.push(chart);
  }

  function renderSalaryChart() {
    const canvas = document.getElementById('chart-salary');
    if (!canvas || salaryData.length === 0) return;

    const textColor = getComputedStyle(document.documentElement).getPropertyValue('--color-slate-700').trim() || '#aebbcf';
    const gridColor = getComputedStyle(document.documentElement).getPropertyValue('--color-slate-200').trim() || '#434c5e';

    const ctx = canvas.getContext('2d');
    const labels = salaryData.map(j => j.company);
    const lows = salaryData.map(j => j.low);
    const highs = salaryData.map(j => j.high);
    const colors = salaryData.map(j => j.status === 'Offer' ? '#bf616a' : '#4c566a');

    const chart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels,
        datasets: [{
          data: highs.map((h, i) => [lows[i], h]),
          backgroundColor: colors,
          barThickness: 12,
          borderRadius: 2,
          borderSkipped: false,
        }]
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            backgroundColor: '#2e3440',
            titleColor: '#eceff4',
            bodyColor: '#d8dee9',
            padding: 8,
            cornerRadius: 4,
            displayColors: false,
            callbacks: {
              label: (ctx) => {
                const raw = ctx.raw;
                return `$${raw[0]}k – $${raw[1]}k`;
              }
            }
          },
        },
        scales: {
          x: {
            grid: { color: gridColor, drawTicks: false },
            border: { color: gridColor },
            ticks: {
              color: textColor,
              callback: v => `$${v}k`,
              padding: 4,
            },
            suggestedMin: Math.max(0, Math.min(...lows) - 10),
            suggestedMax: Math.max(...highs) + 10,
          },
          y: {
            grid: { display: false },
            border: { display: false },
            ticks: { color: textColor, padding: 4 },
          },
        },
      }
    });
    chartInstances.push(chart);
  }

  function parseSalary(str) {
    if (!str) return { low: 0, high: 0, mid: 0 };
    const nums = str.replace(/[,$]/g, '').match(/(\d+)k/gi);
    if (!nums || nums.length < 2) {
      const single = str.replace(/[,$]/g, '').match(/(\d+)k/i);
      if (single) {
        const v = parseInt(single[1]);
        return { low: v, high: v, mid: v };
      }
      return { low: 0, high: 0, mid: 0 };
    }
    const low = parseInt(nums[0]);
    const high = parseInt(nums[1]);
    return { low, high, mid: Math.round((low + high) / 2) };
  }

  function formatDate(d) {
    if (!d) return '';
    try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
    catch { return d; }
  }

  function formatDateTime(d) {
    if (!d) return '';
    try { return new Date(d).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }); }
    catch { return d; }
  }

  function deadlineClass(days) {
    if (days < 0) return 'text-red-600';
    if (days <= 7) return 'text-amber-600';
    return 'text-emerald-600';
  }

  function deadlineLabel(days) {
    if (days < 0) return `${Math.abs(days)}d overdue`;
    if (days === 0) return 'Today';
    return `${days}d`;
  }
</script>

{#if !loaded}
  <Spinner text="Loading dashboard..." />
{:else if jobs.length === 0}
  <!-- Empty state -->
  <div class="text-center py-20 text-slate-400">
    <div class="text-5xl mb-4 opacity-50 flex items-center justify-center">{@html iconSvg("dashboard", 64)}</div>
    <h3 class="text-xl font-semibold text-slate-700 mb-2">Welcome to Waypoint</h3>
    <p class="max-w-sm mx-auto mb-6 leading-relaxed">Your job applications appear here. Use the CLI to add them:</p>
    <pre class="inline-block bg-slate-100 px-5 py-3 rounded-lg text-sm mb-6">waypoint jobs add "Company" "Position"</pre>
    <p class="text-xs">Then reload this page</p>
  </div>
{:else if filteredJobs.length === 0 && hasActiveFilter}
  <div class="text-center py-20 text-slate-400">
    <div class="text-4xl mb-4">{@html iconSvg("search", 48)}</div>
    <h3 class="text-lg font-semibold text-slate-600 mb-1">No jobs match &ldquo;{filter.category}&rdquo;</h3>
    <p class="text-sm">Try selecting a different category filter.</p>
  </div>
{:else}
  <div class="space-y-3">
    <!-- Stat cards -->
    <div class="grid grid-cols-5 gap-2.5">
      <div class="bg-white rounded-lg border border-slate-200 p-3 hover:border-slate-400 transition-colors">
        <div class="text-xs uppercase tracking-wide text-slate-400 font-medium">Total Applications</div>
        <div class="flex items-baseline gap-2 mt-1">
          <span class="text-2xl font-bold text-slate-800 tabular-nums">{filteredJobs.length}</span>
          {#if weekDelta !== 0}
            <span class="text-xs font-semibold {weekDelta > 0 ? 'text-emerald-600' : 'text-red-600'}">{weekDelta > 0 ? '+' : ''}{weekDelta}%</span>
          {/if}
        </div>
      </div>
      <div class="bg-white rounded-lg border border-slate-200 p-3 hover:border-slate-400 transition-colors">
        <div class="text-xs uppercase tracking-wide text-slate-400 font-medium">Active Pipeline</div>
        <div class="flex items-baseline gap-2 mt-1">
          <span class="text-2xl font-bold text-slate-800 tabular-nums">{activePipeline}</span>
          <span class="text-xs text-slate-400">{notApplied} waiting, {applied} in review</span>
        </div>
      </div>
      <div class="bg-white rounded-lg border border-slate-200 p-3 hover:border-slate-400 transition-colors">
        <div class="text-xs uppercase tracking-wide text-slate-400 font-medium">Offers</div>
        <div class="flex items-baseline gap-2 mt-1">
          <span class="text-2xl font-bold text-slate-600 tabular-nums">{offers}</span>
          <span class="text-xs text-slate-400">{offerRate}% conversion</span>
        </div>
      </div>
      <div class="bg-white rounded-lg border border-slate-200 p-3 hover:border-slate-400 transition-colors">
        <div class="text-xs uppercase tracking-wide text-slate-400 font-medium">Response Rate</div>
        <div class="flex items-baseline gap-2 mt-1">
          <span class="text-2xl font-bold text-slate-800 tabular-nums">{responseRate}%</span>
        </div>
      </div>
      <div class="bg-white rounded-lg border border-slate-200 p-3 hover:border-slate-400 transition-colors">
        <div class="text-xs uppercase tracking-wide text-slate-400 font-medium">This Week</div>
        <div class="flex items-baseline gap-2 mt-1">
          <span class="text-2xl font-bold text-slate-800 tabular-nums">{actionsThisWeek}</span>
          <span class="text-xs text-slate-400">{actionsThisWeek === 1 ? 'action' : 'actions'}</span>
        </div>
      </div>
    </div>

    <!-- Charts row -->
    <div class="grid grid-cols-2 gap-3">
      {#if pipelineData.length > 0}
        <div class="bg-white rounded-lg border border-slate-200 p-3">
          <h4 class="text-xs uppercase tracking-wide text-slate-400 font-medium mb-2">Pipeline</h4>
          <div class="h-48"><canvas id="chart-pipeline"></canvas></div>
        </div>
      {/if}
      {#if salaryData.length > 0}
        <div class="bg-white rounded-lg border border-slate-200 p-3">
          <h4 class="text-xs uppercase tracking-wide text-slate-400 font-medium mb-2">Salary Range</h4>
          <div class="h-64"><canvas id="chart-salary"></canvas></div>
        </div>
      {/if}
    </div>

    <!-- Two-column layout -->
    <div class="grid grid-cols-2 gap-3">
      <!-- Left column -->
      <div class="flex flex-col gap-3">
        <!-- Stale / needs follow-up -->
        {#if staleApps.length > 0}
          <div class="bg-white rounded-lg border border-amber-200 p-3">
            <h4 class="text-xs uppercase tracking-wide text-slate-400 font-medium mb-2">
              Needs Follow-up
              <span class="ml-1.5 bg-amber-500 text-white text-[10px] font-bold rounded-full px-1.5 py-0.5 align-middle">{staleApps.length}</span>
            </h4>
            <div class="max-h-[200px] overflow-y-auto space-y-1">
              {#each staleApps as app}
                <button
                  class="w-full flex justify-between items-baseline py-1.5 border-b border-slate-100 last:border-0 text-left cursor-pointer bg-transparent hover:bg-slate-50 px-1 rounded transition-colors"
                  onclick={() => router.navigate('/job/' + app.id)}
                >
                  <div>
                    <span class="text-sm font-semibold text-slate-700">{app.company}</span>
                    <span class="text-xs text-slate-400 ml-1">{app.position}</span>
                  </div>
                  <div class="text-right shrink-0 ml-2">
                    <span class="text-xs font-semibold {app.daysStale > 30 ? 'text-red-600' : 'text-amber-600'}">{app.daysStale}d</span>
                  </div>
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Upcoming deadlines -->
        {#if upcoming.length > 0}
          <div class="bg-white rounded-lg border border-slate-200 p-3">
            <h4 class="text-xs uppercase tracking-wide text-slate-400 font-medium mb-2">Upcoming Deadlines</h4>
            <div class="space-y-1">
              {#each upcoming as job}
                <button
                  class="w-full flex justify-between items-baseline py-1.5 border-b border-slate-100 last:border-0 text-left cursor-pointer bg-transparent hover:bg-slate-50 px-1 rounded transition-colors"
                  onclick={() => router.navigate('/job/' + job.id)}
                >
                  <div>
                    <span class="text-sm font-semibold text-slate-700">{job.company}</span>
                    <span class="text-xs text-slate-400 ml-1">{job.position}</span>
                  </div>
                  <div class="text-right shrink-0 ml-2">
                    <span class="text-xs font-semibold {deadlineClass(job.daysLeft)}">{deadlineLabel(job.daysLeft)}</span>
                  </div>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <!-- Right column -->
      <div class="flex flex-col gap-3">
        <!-- Category breakdown -->
        <div class="bg-white rounded-lg border border-slate-200 p-3">
          <h4 class="text-xs uppercase tracking-wide text-slate-400 font-medium mb-2">By Category</h4>
          <div class="overflow-x-auto">
            <table class="w-full text-xs">
              <thead>
                <tr class="border-b border-slate-200">
                  <th class="text-left py-1.5 pr-2 uppercase tracking-wide text-slate-400 font-medium">Category</th>
                  <th class="text-right py-1.5 px-2 uppercase tracking-wide text-slate-400 font-medium">Total</th>
                  <th class="text-right py-1.5 px-2 uppercase tracking-wide text-slate-400 font-medium">Applied</th>
                  <th class="text-right py-1.5 px-2 uppercase tracking-wide text-slate-400 font-medium">Offer</th>
                  <th class="text-right py-1.5 px-2 uppercase tracking-wide text-slate-400 font-medium">Lost</th>
                </tr>
              </thead>
              <tbody>
                {#each catCounts as cat, i}
                  {@const catJobs = jobs.filter(j => (j.category || 'General') === cat.label)}
                  <tr class="border-b border-slate-50 hover:bg-slate-50">
                    <td class="py-1.5 pr-2 font-medium text-slate-700">{cat.label}</td>
                    <td class="py-1.5 px-2 text-right tabular-nums {i === 0 ? 'text-slate-800 font-bold' : 'text-slate-600'}">{cat.value}</td>
                    <td class="py-1.5 px-2 text-right tabular-nums text-slate-600">{catJobs.filter(j => j.status === 'Applied').length || '-'}</td>
                    <td class="py-1.5 px-2 text-right tabular-nums text-slate-600">{catJobs.filter(j => j.status === 'Offer').length || '-'}</td>
                    <td class="py-1.5 px-2 text-right tabular-nums text-slate-600">{(catJobs.filter(j => j.status === 'Rejected').length + catJobs.filter(j => j.status === 'Withdrawn').length) || '-'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>

        <!-- Recent activity -->
        <div class="bg-white rounded-lg border border-slate-200 p-3">
          <h4 class="text-xs uppercase tracking-wide text-slate-400 font-medium mb-2">Recent Activity</h4>
          {#if recentActivity.length === 0}
            <p class="text-xs text-slate-400">No activity yet.</p>
          {:else}
            <div class="space-y-1">
              {#each recentActivity as h}
                {@const job = jobs.find(j => j.id === h.jobId)}
                {#if job}
                  <div class="flex justify-between items-baseline py-1 border-b border-slate-100 last:border-0">
                    <span class="text-xs text-slate-600">
                      <button
                        class="font-semibold text-slate-700 hover:text-slate-500 cursor-pointer bg-transparent border-none p-0 text-xs"
                        onclick={() => router.navigate('/job/' + job.id)}
                      >{job.company}</button>
                      {h.from} → {h.to}
                    </span>
                    <span class="text-[11px] text-slate-400 shrink-0 ml-2">{formatDateTime(h.timestamp)}</span>
                  </div>
                {/if}
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}
