/* === Dashboard View (Tufte-compliant) === */
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
    const withdrawn = statusCounts['Withdrawn'] || 0;
    const activePipeline = notApplied + applied;
    const responded = offers + rejected;
    const responseRate = applied > 0 ? Math.round(responded / applied * 100) : 0;
    const offerRate = applied > 0 ? Math.round(offers / applied * 100) : 0;

    if (jobs.length === 0) {
      container.innerHTML = `
        <div class="empty-state" style="padding:80px 24px">
          <div class="empty-icon">
            <svg viewBox="0 0 100 100" width="80" height="80" aria-hidden="true" style="opacity:0.5">
              <g fill="none" stroke="currentColor" stroke-linecap="round">
                <circle cx="50" cy="50" r="32" stroke-width="4"/>
                <circle cx="50" cy="50" r="16" stroke-width="1.5" stroke-dasharray="4 6" opacity="0.4"/>
                <circle cx="50" cy="18" r="4" fill="currentColor" stroke="none"/>
                <circle cx="82" cy="50" r="4" fill="currentColor" stroke="none"/>
                <circle cx="50" cy="82" r="4" fill="currentColor" stroke="none"/>
                <circle cx="18" cy="50" r="4" fill="currentColor" stroke="none"/>
                <polygon points="50,40 60,50 50,60 40,50" fill="currentColor" stroke="none"/>
                <path d="M 50 18 A 32 32 0 0 1 82 50" stroke-width="2.5" opacity="0.5"/>
                <circle cx="50" cy="50" r="2" fill="currentColor" stroke="none"/>
              </g>
            </svg>
          </div>
          <h3 style="font-size:22px;margin-bottom:8px">Welcome to Waypoint</h3>
          <p style="font-size:15px;max-width:400px;margin:0 auto 24px;line-height:1.6">Your job applications appear here. Use the CLI to add them:</p>
          <pre style="background:var(--bg-secondary);padding:12px 20px;border-radius:8px;font-size:14px;display:inline-block;margin-bottom:24px">waypoint jobs add "Company" "Position"</pre>
          <p class="text-muted" style="font-size:13px">Then reload this page</p>
        </div>
      `;
      document.getElementById('view-title').textContent = 'Dashboard';
      return;
    }

    // === Derived analytics ===

    // Monthly sparkline for stat card
    const months = Object.keys(monthlyCounts).sort((a, b) => {
      const [mA, yA] = a.split(' '); const [mB, yB] = b.split(' ');
      return yA !== yB ? yA - yB : monthNames.indexOf(mA) - monthNames.indexOf(mB);
    });
    const monthValues = months.map(m => monthlyCounts[m]);
    const sparklineSvg = Dashboard._sparklineSVG(monthValues, 72, 20);

    // Week-over-week delta
    const now = new Date();
    const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
    const twoWeeksAgo = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000);
    let thisWeek = 0, lastWeek = 0;
    jobs.forEach(j => {
      const d = j.appliedDate ? new Date(j.appliedDate) : null;
      if (d && d >= weekAgo) thisWeek++;
      else if (d && d >= twoWeeksAgo) lastWeek++;
    });
    const weekDelta = lastWeek > 0 ? Math.round((thisWeek - lastWeek) / lastWeek * 100) : (thisWeek > 0 ? 100 : 0);
    const deltaHtml = weekDelta !== 0
      ? `<span class="stat-delta ${weekDelta > 0 ? 'positive' : 'negative'}">${weekDelta > 0 ? '+' : ''}${weekDelta}%</span>`
      : '';

    // Pipeline velocity: median days from Applied -> Offer/Rejected
    const transitionTimes = [];
    const jobAppliedDate = {};
    allHistory.forEach(h => {
      if (h.action === 'Status' && h.from === 'Applied' && (h.to === 'Offer' || h.to === 'Rejected')) {
        const job = jobs.find(j => j.id === h.jobId);
        if (job && job.appliedDate) {
          const applied = new Date(job.appliedDate);
          const resolved = new Date(h.timestamp);
          const days = Math.round((resolved - applied) / (1000 * 60 * 60 * 24));
          if (days >= 0) transitionTimes.push(days);
        }
      }
    });
    const medianResponse = transitionTimes.length > 0
      ? transitionTimes.sort((a, b) => a - b)[Math.floor(transitionTimes.length / 2)]
      : null;

    // Actions this week (from history)
    const actionsThisWeek = allHistory.filter(h => new Date(h.timestamp) >= weekAgo).length;

    // Stale applications: Applied > 14 days with no response
    const staleThreshold = 14;
    const staleApps = jobs.filter(j => {
      if (j.status !== 'Applied' || !j.appliedDate) return false;
      const days = Math.round((now - new Date(j.appliedDate)) / (1000 * 60 * 60 * 24));
      return days > staleThreshold;
    }).map(j => ({
      ...j,
      daysStale: Math.round((now - new Date(j.appliedDate)) / (1000 * 60 * 60 * 24))
    })).sort((a, b) => b.daysStale - a.daysStale);

    // Upcoming deadlines
    const upcoming = jobs
      .filter(j => j.date && j.status !== 'Offer' && j.status !== 'Withdrawn' && j.status !== 'Rejected')
      .map(j => {
        const dd = new Date(j.date);
        const daysLeft = Math.ceil((dd - now) / (1000 * 60 * 60 * 24));
        return { ...j, daysLeft };
      })
      .sort((a, b) => a.daysLeft - b.daysLeft);

    // Category breakdown table
    const catPairs = Object.entries(catCounts)
      .map(([label, value]) => ({ label: label || 'Uncategorized', value }))
      .sort((a, b) => b.value - a.value);

    const pipelineStages = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];
    const catStatusData = {};
    catPairs.forEach(cat => {
      const catJobs = jobs.filter(j => (j.category || 'Uncategorized') === cat.label);
      catStatusData[cat.label] = {};
      pipelineStages.forEach(s => {
        catStatusData[cat.label][s] = catJobs.filter(j => j.status === s).length;
      });
    });

    // Salary data for dot plot
    const salaryJobs = jobs.filter(j => j.salary && j.salary.trim()).map(j => {
      const parsed = Dashboard._parseSalary(j.salary);
      return { ...j, salaryMid: parsed.mid, salaryLow: parsed.low, salaryHigh: parsed.high };
    }).filter(j => j.salaryMid > 0).sort((a, b) => b.salaryMid - a.salaryMid);

    // Pipeline conversion rates
    const pipelineData = [
      { stage: 'Not Applied', count: notApplied },
      { stage: 'Applied', count: applied },
      { stage: 'Offer', count: offers },
      { stage: 'Rejected', count: rejected },
      { stage: 'Withdrawn', count: withdrawn },
    ].filter(d => d.count > 0);

    // === RENDER ===
    container.innerHTML = `
      <div class="dashboard-stats">
        <div class="stat-card">
          <div class="stat-header"><span class="stat-label">Total Applications</span></div>
          <div class="stat-row"><span class="stat-value">${jobs.length}</span>${sparklineSvg}${deltaHtml}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header"><span class="stat-label">Active Pipeline</span></div>
          <div class="stat-row"><span class="stat-value">${activePipeline}</span><span class="stat-detail">${notApplied} waiting, ${applied} in review</span></div>
        </div>
        <div class="stat-card">
          <div class="stat-header"><span class="stat-label">Offers</span></div>
          <div class="stat-row"><span class="stat-value stat-value--accent">${offers}</span><span class="stat-detail">${offerRate}% conversion from applied</span></div>
        </div>
        <div class="stat-card">
          <div class="stat-header"><span class="stat-label">Response Rate</span></div>
          <div class="stat-row"><span class="stat-value">${responseRate}%</span><span class="stat-detail">${medianResponse !== null ? medianResponse + 'd median response' : 'No data yet'}</span></div>
        </div>
        <div class="stat-card">
          <div class="stat-header"><span class="stat-label">This Week</span></div>
          <div class="stat-row"><span class="stat-value">${actionsThisWeek}</span><span class="stat-detail">${actionsThisWeek === 1 ? 'action' : 'actions'}</span></div>
        </div>
      </div>

      <div class="dashboard-two-col">
        <div class="dashboard-col-left">
          <div class="chart-card">
            <h4>Pipeline</h4>
            <canvas id="chart-pipeline"></canvas>
          </div>
          ${staleApps.length > 0 ? `
          <div class="chart-card chart-card--alert">
            <h4>Needs Follow-up <span class="alert-badge">${staleApps.length}</span></h4>
            <div class="stale-list" id="stale-list"></div>
          </div>` : ''}
          ${salaryJobs.length > 0 ? `
          <div class="chart-card">
            <h4>Salary Range</h4>
            <canvas id="chart-salary"></canvas>
          </div>` : ''}
        </div>
        <div class="dashboard-col-right">
          <div class="chart-card">
            <h4>By Category</h4>
            <div class="cat-table-wrap">
              <table class="cat-table" id="cat-table">
                <thead>
                  <tr>
                    <th class="cat-th-left">Category</th>
                    <th>Total</th>
                    <th>Waiting</th>
                    <th>Applied</th>
                    <th>Offer</th>
                    <th>Lost</th>
                  </tr>
                </thead>
                <tbody id="cat-table-body"></tbody>
              </table>
            </div>
          </div>
          ${upcoming.length > 0 ? `
          <div class="chart-card">
            <h4>Upcoming Deadlines</h4>
            <div id="deadline-list"></div>
          </div>` : ''}
          <div class="chart-card">
            <h4>Recent Activity</h4>
            <div id="recent-activity"></div>
          </div>
        </div>
      </div>
    `;

    // --- Populate stale applications ---
    if (staleApps.length > 0) {
      const staleList = document.getElementById('stale-list');
      staleList.innerHTML = staleApps.map(j => {
        const urgency = j.daysStale > 60 ? 'stale-critical' : j.daysStale > 30 ? 'stale-warning' : '';
        return `<div class="stale-item ${urgency}">
          <div class="stale-main">
            <a href="/job/${j.id}" class="stale-company recent-job-link" data-job-id="${j.id}">${UI.escapeHtml(j.company)}</a>
            <span class="stale-position">${UI.escapeHtml(j.position)}</span>
          </div>
          <div class="stale-meta">
            <span class="stale-days">${j.daysStale}d</span>
            <span class="stale-since">since ${j.appliedDate}</span>
          </div>
        </div>`;
      }).join('');

      staleList.querySelectorAll('.recent-job-link').forEach(link => {
        link.addEventListener('click', (e) => {
          e.preventDefault();
          App.currentJobId = parseInt(link.dataset.jobId, 10);
          App.switchView('job');
        });
      });
    }

    // --- Populate category table ---
    const catTableBody = document.getElementById('cat-table-body');
    catTableBody.innerHTML = catPairs.map((cat, idx) => {
      const sd = catStatusData[cat.label];
      const isTop = idx === 0;
      const lost = (sd['Rejected'] || 0) + (sd['Withdrawn'] || 0);
      return `<tr>
        <td class="cat-td-left">${cat.label}</td>
        <td class="cat-td-num ${isTop ? 'cat-td-accent' : ''}">${cat.value}</td>
        <td class="cat-td-num">${sd['Not Applied'] || '-'}</td>
        <td class="cat-td-num">${sd['Applied'] || '-'}</td>
        <td class="cat-td-num">${sd['Offer'] || '-'}</td>
        <td class="cat-td-num">${lost || '-'}</td>
      </tr>`;
    }).join('');

    // --- Populate upcoming deadlines ---
    if (upcoming.length > 0) {
      const deadlineList = document.getElementById('deadline-list');
      deadlineList.innerHTML = upcoming.map(j => {
        const urgent = j.daysLeft <= 7 && j.daysLeft >= 0;
        const past = j.daysLeft < 0;
        const daysLabel = past ? `${Math.abs(j.daysLeft)}d overdue` : j.daysLeft === 0 ? 'Today' : `${j.daysLeft}d`;
        const daysClass = past ? 'deadline-past' : urgent ? 'deadline-urgent' : 'deadline-ok';
        return `<div class="deadline-item">
          <div class="deadline-main">
            <a href="/job/${j.id}" class="deadline-company recent-job-link" data-job-id="${j.id}">${UI.escapeHtml(j.company)}</a>
            <span class="deadline-position">${UI.escapeHtml(j.position)}</span>
          </div>
          <div class="deadline-meta">
            <span class="deadline-date">${j.date}</span>
            <span class="deadline-days ${daysClass}">${daysLabel}</span>
          </div>
        </div>`;
      }).join('');

      deadlineList.querySelectorAll('.recent-job-link').forEach(link => {
        link.addEventListener('click', (e) => {
          e.preventDefault();
          App.currentJobId = parseInt(link.dataset.jobId, 10);
          App.switchView('job');
        });
      });
    }

    // --- Populate recent activity ---
    const recentEl = document.getElementById('recent-activity');
    const jobMap = {};
    jobs.forEach(j => jobMap[j.id] = j);
    const recent = allHistory.filter(h => h.action === 'Status' && h.from).slice(0, 8);
    if (recent.length === 0) {
      // Fallback to all history
      const allRecent = allHistory.slice(0, 8);
      if (allRecent.length === 0) {
        recentEl.innerHTML = '<div class="text-muted text-sm">No activity yet.</div>';
      } else {
        recentEl.innerHTML = allRecent.map(h => {
          const j = jobMap[h.jobId];
          if (!j) return '';
          const actionIcon = icon('edit', 14);
          return `<div class="recent-item">
            <span>${actionIcon} <a href="/job/${j.id}" class="recent-job-link" data-job-id="${j.id}"><strong>${UI.escapeHtml(j.company)}</strong></a> ${h.action}${h.to ? ' to ' + h.to : ''}</span>
            <span class="text-muted">${UI.formatDateTime(h.timestamp)}</span>
          </div>`;
        }).join('');
      }
    } else {
      recentEl.innerHTML = recent.map(h => {
        const j = jobMap[h.jobId];
        if (!j) return '';
        const arrow = h.to === 'Offer' ? '→ ' + h.to : h.to === 'Rejected' ? '→ ' + h.to : '→ ' + h.to;
        const actionIcon = h.to === 'Offer' ? icon('check', 14) : h.to === 'Rejected' ? icon('x', 14) : h.to === 'Withdrawn' ? icon('minus', 14) : icon('arrow-right', 14);
        return `<div class="recent-item">
          <span>${actionIcon} <a href="/job/${j.id}" class="recent-job-link" data-job-id="${j.id}"><strong>${UI.escapeHtml(j.company)}</strong></a> ${h.from} ${arrow}</span>
          <span class="text-muted">${UI.formatDateTime(h.timestamp)}</span>
        </div>`;
      }).join('');
    }

    recentEl.querySelectorAll('.recent-job-link').forEach(link => {
      link.addEventListener('click', (e) => {
        e.preventDefault();
        App.currentJobId = parseInt(link.dataset.jobId, 10);
        App.switchView('job');
      });
    });

    document.getElementById('view-title').textContent = 'Dashboard';

    // Render charts after DOM update
    setTimeout(() => this.renderCharts(pipelineData, salaryJobs), 100);
  },

  /**
   * Parse salary string like "$160k-$195k" into {low, high, mid} in thousands.
   */
  _parseSalary(str) {
    const nums = str.replace(/[,$]/g, '').match(/(\d+)k/gi);
    if (!nums || nums.length < 2) {
      // Try single number
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
  },

  /**
   * Generate an inline sparkline SVG from a values array.
   */
  _sparklineSVG(values, w, h) {
    if (!values || values.length < 2) return '';
    const max = Math.max(...values);
    const min = Math.min(...values);
    const range = max - min || 1;
    const pad = 2;
    const plotH = h - pad * 2;
    const step = w / (values.length - 1);
    const points = values.map((v, i) => {
      const x = i * step;
      const y = pad + plotH - ((v - min) / range) * plotH;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    const lastX = ((values.length - 1) * step).toFixed(1);
    const lastY = (pad + plotH - ((values[values.length - 1] - min) / range) * plotH).toFixed(1);
    return `<svg class="stat-sparkline" viewBox="0 0 ${w} ${h}" width="${w}" height="${h}" aria-hidden="true">` +
      `<polyline fill="none" stroke="var(--text-muted)" stroke-width="1" points="${points.join(' ')}"/>` +
      `<circle cx="${lastX}" cy="${lastY}" r="2" fill="var(--accent)"/>` +
      `</svg>`;
  },

  renderCharts(pipelineData, salaryJobs) {
    Object.values(this.charts).forEach(c => { try { c.destroy(); } catch {} });
    this.charts = {};

    if (typeof Chart === 'undefined') return;

    const tufteFont = { family: "'Inter', 'Helvetica Neue', Arial, sans-serif" };
    const gridColor = 'rgba(0,0,0,0.04)';
    const axisColor = 'rgba(0,0,0,0.15)';
    const tickFont = { ...tufteFont, size: 11, color: '#6b665d' };
    const labelFont = { ...tufteFont, size: 12 };
    const accentColor = '#BF616A';
    const contextColor = '#4C566A';

    // --- 1. PIPELINE: horizontal bar in funnel order, with conversion rates ---
    const pipelineLabels = pipelineData.map(d => d.stage);
    const pipelineValues = pipelineData.map(d => d.count);

    // Color: accent on Offer, context on rest
    const pipelineColors = pipelineLabels.map(s =>
      s === 'Offer' ? accentColor : contextColor
    );

    // Build labels with conversion rates
    const appliedCount = pipelineData.find(d => d.stage === 'Applied')?.count || 0;
    const pipelineTickLabels = pipelineLabels.map((stage, i) => {
      const count = pipelineValues[i];
      if (stage === 'Offer' && appliedCount > 0) {
        return `${stage}  (${Math.round(count / appliedCount * 100)}%)`;
      }
      if (stage === 'Rejected' && appliedCount > 0) {
        return `${stage}  (${Math.round(count / appliedCount * 100)}%)`;
      }
      return stage;
    });

    const pipelineCtx = document.getElementById('chart-pipeline');
    if (pipelineCtx) {
      this.charts.pipeline = new Chart(pipelineCtx, {
        type: 'bar',
        data: {
          labels: pipelineTickLabels,
          datasets: [{
            data: pipelineValues,
            backgroundColor: pipelineColors,
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
              backgroundColor: '#2E3440',
              titleFont: labelFont,
              bodyFont: labelFont,
              padding: 8,
              cornerRadius: 4,
              displayColors: false,
            },
          },
          scales: {
            x: {
              beginAtZero: true,
              grid: { color: gridColor, drawTicks: false },
              border: { color: axisColor },
              ticks: { ...tickFont, stepSize: 1, padding: 4 },
            },
            y: {
              grid: { display: false },
              border: { display: false },
              ticks: { ...tickFont, padding: 4 },
            },
          },
        }
      });
    }

    // --- 2. SALARY RANGE: horizontal floating bar (low-high per job) ---
    if (salaryJobs.length > 0) {
      const salaryCtx = document.getElementById('chart-salary');
      if (salaryCtx) {
        const salaryLabels = salaryJobs.map(j => j.company);
        const salaryLows = salaryJobs.map(j => j.salaryLow);
        const salaryHighs = salaryJobs.map(j => j.salaryHigh);

        // Color: accent on offers, context on the rest
        const salaryColors = salaryJobs.map(j =>
          j.status === 'Offer' ? accentColor : contextColor
        );

        this.charts.salary = new Chart(salaryCtx, {
          type: 'bar',
          data: {
            labels: salaryLabels,
            datasets: [{
              data: salaryHighs.map((h, i) => [salaryLows[i], h]),
              backgroundColor: salaryColors,
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
                backgroundColor: '#2E3440',
                titleFont: labelFont,
                bodyFont: labelFont,
                padding: 8,
                cornerRadius: 4,
                displayColors: false,
                callbacks: {
                  label: function(ctx) {
                    const raw = ctx.raw;
                    return `$${raw[0]}k – $${raw[1]}k`;
                  }
                }
              },
            },
            scales: {
              x: {
                grid: { color: gridColor, drawTicks: false },
                border: { color: axisColor },
                ticks: {
                  ...tickFont,
                  callback: v => `$${v}k`,
                  padding: 4,
                },
                // Range frame: just past the data
                suggestedMin: Math.max(0, Math.min(...salaryLows) - 10),
                suggestedMax: Math.max(...salaryHighs) + 10,
              },
              y: {
                grid: { display: false },
                border: { display: false },
                ticks: { ...tickFont, padding: 4 },
              },
            },
          }
        });
      }
    }
  },
};
