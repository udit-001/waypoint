/* === Dashboard View (read-only) === */
const Dashboard = {
  charts: {},

  async render() {
    const container = document.getElementById('view-dashboard');
    const jobs = await DB.getJobs();
    const allHistory = await DB.getHistory();

    const statusCounts = {};
    const catCounts = {};
    const monthlyCounts = {};
    const today = new Date();
    const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

    jobs.forEach(j => {
      statusCounts[j.status] = (statusCounts[j.status] || 0) + 1;
      catCounts[j.category] = (catCounts[j.category] || 0) + 1;
      const dstr = j.appliedDate || j.date;
      if (dstr) {
        const d = new Date(dstr);
        const key = `${monthNames[d.getMonth()]} ${d.getFullYear()}`;
        monthlyCounts[key] = (monthlyCounts[key] || 0) + 1;
      }
    });

    const notApplied = statusCounts['Not Applied'] || 0;
    const applied = statusCounts['Applied'] || 0;
    const offers = statusCounts['Offer'] || 0;
    const rejected = statusCounts['Rejected'] || 0;
    const activePipeline = notApplied + applied;
    const responseRate = jobs.length ? Math.round((offers) / jobs.length * 100) : 0;

    if (jobs.length === 0) {
      container.innerHTML = `
        <div class="empty-state" style="padding:80px 24px">
          <div class="empty-icon">${icon('briefcase', 64)}</div>
          <h3 style="font-size:22px;margin-bottom:8px">Welcome to Job Tracker</h3>
          <p style="font-size:15px;max-width:400px;margin:0 auto 24px;line-height:1.6">Your job applications appear here. Use the CLI to add them:</p>
          <pre style="background:var(--bg-secondary);padding:12px 20px;border-radius:8px;font-size:14px;display:inline-block;margin-bottom:24px">job-tracker add "Company" "Position"</pre>
          <p class="text-muted" style="font-size:13px">Then reload this page</p>
        </div>
      `;
      document.getElementById('view-title').textContent = 'Dashboard';
      return;
    }

    container.innerHTML = `
      <div class="dashboard-stats">
        <div class="stat-card"><div class="stat-value">${jobs.length}</div><div class="stat-label">Total Applications</div></div>
        <div class="stat-card"><div class="stat-value">${activePipeline}</div><div class="stat-label">Active Pipeline</div></div>
        <div class="stat-card"><div class="stat-value">${offers}</div><div class="stat-label">Offers Received</div></div>
        <div class="stat-card"><div class="stat-value">${responseRate}%</div><div class="stat-label">Response Rate</div></div>
        <div class="stat-card"><div class="stat-value">${rejected}</div><div class="stat-label">Rejected</div></div>
      </div>
      <div class="dashboard-charts">
        <div class="chart-card"><h4>Status Breakdown</h4><canvas id="chart-status"></canvas></div>
        <div class="chart-card"><h4>By Category</h4><canvas id="chart-category"></canvas></div>
        <div class="chart-card"><h4>Monthly Applications</h4><canvas id="chart-monthly"></canvas></div>
      </div>
      <div class="dashboard-recent">
        <h4>Recent Activity</h4>
        <div id="recent-activity"></div>
      </div>
    `;

    // Recent activity
    const recentEl = document.getElementById('recent-activity');
    const jobMap = {};
    jobs.forEach(j => jobMap[j.id] = j);
    const recent = allHistory.slice(0, 10);
    if (recent.length === 0) {
      recentEl.innerHTML = '<div class="text-muted text-sm">No activity yet. Start by adding a job via the CLI!</div>';
    } else {
      recentEl.innerHTML = recent.map(h => {
        const j = jobMap[h.jobId];
        if (!j) return '';
        const actionIcon = h.action === 'Created' ? icon('plus', 14) : h.action === 'Status' ? icon('pin', 14) : h.action === 'Deleted' ? icon('trash', 14) : icon('edit', 14);
        return `<div class="recent-item">
          <span>${actionIcon} <a href="/job/${j.id}" class="recent-job-link" data-job-id="${j.id}"><strong>${UI.escapeHtml(j.company)}</strong></a> - ${h.action}${h.to ? ' to ' + h.to : ''}</span>
          <span class="text-muted">${UI.formatDateTime(h.timestamp)}</span>
        </div>`;
      }).join('');

      recentEl.querySelectorAll('.recent-job-link').forEach(link => {
        link.addEventListener('click', (e) => {
          e.preventDefault();
          App.currentJobId = parseInt(link.dataset.jobId, 10);
          App.switchView('job');
        });
      });
    }

    document.getElementById('view-title').textContent = 'Dashboard';

    // Render charts after DOM update
    setTimeout(() => this.renderCharts(statusCounts, catCounts, monthlyCounts), 100);
  },

  renderCharts(statusCounts, catCounts, monthlyCounts) {
    // Destroy existing charts
    Object.values(this.charts).forEach(c => { try { c.destroy(); } catch {} });
    this.charts = {};

    const statusCtx = document.getElementById('chart-status');
    const catCtx = document.getElementById('chart-category');
    const monthlyCtx = document.getElementById('chart-monthly');

    if (!statusCtx || !catCtx || !monthlyCtx) return;

    const statusOrder = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];
    const statusColors = ['#81A1C1', '#5E81AC', '#A3BE8C', '#BF616A', '#4C566A'];

    if (typeof Chart !== 'undefined') {
      const compactChart = {
        responsive: true,
        maintainAspectRatio: false,
      };

      this.charts.status = new Chart(statusCtx, {
        type: 'doughnut',
        data: {
          labels: statusOrder.filter(s => statusCounts[s]),
          datasets: [{
            data: statusOrder.filter(s => statusCounts[s]).map(s => statusCounts[s]),
            backgroundColor: statusOrder.filter(s => statusCounts[s]).map(s => statusColors[statusOrder.indexOf(s)])
          }]
        },
        options: {
          ...compactChart,
          plugins: { legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 11 } } } },
        }
      });

      this.charts.category = new Chart(catCtx, {
        type: 'bar',
        data: {
          labels: Object.keys(catCounts),
          datasets: [{
            label: 'Applications',
            data: Object.values(catCounts),
            backgroundColor: '#88C0D0'
          }]
        },
        options: {
          ...compactChart,
          plugins: { legend: { display: false } },
          scales: { y: { beginAtZero: true, ticks: { stepSize: 1, font: { size: 10 } } }, x: { ticks: { font: { size: 10 } } } }
        }
      });

      const months = Object.keys(monthlyCounts).sort((a, b) => {
        const [mA, yA] = a.split(' '); const [mB, yB] = b.split(' ');
        return yA !== yB ? yA - yB : monthNames.indexOf(mA) - monthNames.indexOf(mB);
      });

      this.charts.monthly = new Chart(monthlyCtx, {
        type: 'line',
        data: {
          labels: months,
          datasets: [{
            label: 'Applications',
            data: months.map(m => monthlyCounts[m]),
            borderColor: '#88C0D0',
            backgroundColor: 'rgba(136,192,208,0.15)',
            fill: true,
            tension: 0.3
          }]
        },
        options: {
          ...compactChart,
          plugins: { legend: { display: false } },
          scales: { y: { beginAtZero: true, ticks: { stepSize: 1, font: { size: 10 } } }, x: { ticks: { font: { size: 10 } } } }
        }
      });
    }
  },
};
