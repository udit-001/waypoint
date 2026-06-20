/* === Main Application (read-only, URL-routed) === */
const App = {
  currentView: null,
  currentCategory: 'all',
  currentJobId: null,
  searchQuery: '',
  advancedFilters: null,
  tableCategoryFilter: null,

  _views: ['dashboard', 'kanban', 'table', 'timeline', 'skills', 'generated', 'settings', 'job'],

  async init() {
    const settings = await DB.getSettings();
    const savedTheme = localStorage.getItem('jobtracker_theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const theme = savedTheme || (prefersDark ? 'dark' : settings.theme || 'light');
    document.documentElement.dataset.theme = theme;
    document.getElementById('theme-toggle').innerHTML = icon(theme === 'dark' ? 'sun' : 'moon', 18);

    // Font init
    const savedFont = localStorage.getItem('waypoint_font');
    if (savedFont) {
      document.documentElement.dataset.font = savedFont;
    }

    await UI.renderCategories();
    UI.init();
    Skills.init();
    Notes.initPreview();

    window.addEventListener('popstate', async (e) => {
      const route = this._getViewFromPath();
      await this._switchToView(route, false);
    });

    const route = this._getViewFromPath() || settings.defaultView || 'dashboard';
    await this._switchToView(route, false);

    document.getElementById('job-save-btn').addEventListener('click', () => {
      UI.showToast('Use the CLI to manage jobs.', 'info');
    });
    document.getElementById('job-reminder').addEventListener('change', (e) => {
      document.getElementById('job-reminder-date').style.display = e.target.checked ? 'block' : 'none';
    });
  },

  _getViewFromPath() {
    const path = window.location.pathname.replace(/^\/+/, '').replace(/\/+$/, '');
    const jobMatch = path.match(/^job\/(\d+)$/);
    if (jobMatch) {
      this._pendingJobId = parseInt(jobMatch[1], 10);
      return 'job';
    }
    if (this._views.includes(path)) return path;
    return null;
  },

  async _switchToView(view, pushHistory) {
    if (!view) return;

    // Allow re-entering 'job' view for different jobs
    if (view !== 'job' && this.currentView === view) return;

    const prevView = this.currentView;
    this.currentView = view;

    // Resolve pending job ID from URL
    if (view === 'job' && this._pendingJobId) {
      this.currentJobId = this._pendingJobId;
      this._pendingJobId = null;
    }

    if (pushHistory) {
      let path;
      if (view === 'dashboard') path = '/';
      else if (view === 'job') path = '/job/' + this.currentJobId;
      else path = '/' + view;
      history.pushState({ view }, '', path);
    }

    // Title
    const titles = {
      dashboard: 'Dashboard', kanban: 'Kanban Board', table: 'Table View',
      timeline: 'Timeline', skills: 'AI Integration', generated: 'Generated Content',
      settings: 'Settings', job: this.currentJobId ? 'Job #' + this.currentJobId : 'Job Detail',
    };
    document.title = (titles[view] || 'Dashboard') + ' — Waypoint';

    // View panes
    document.querySelectorAll('.view-pane').forEach(p => p.classList.remove('active'));
    let pane = document.getElementById('view-' + view);
    if (!pane && view === 'job') {
      pane = document.createElement('div');
      pane.className = 'view-pane';
      pane.id = 'view-job';
      document.getElementById('view-container').appendChild(pane);
    }
    if (pane) pane.classList.add('active');

    // Nav items (skip for job detail — no active nav)
    if (view !== 'job') {
      document.querySelectorAll('.nav-item[data-view]').forEach(n => n.classList.remove('active'));
      document.querySelectorAll(`.nav-item[data-view="${view}"]`).forEach(n => n.classList.add('active'));
    }

    // View toggles
    const toggles = document.getElementById('view-toggles');
    toggles.style.display = (view === 'dashboard' || view === 'kanban' || view === 'table' || view === 'timeline') ? 'flex' : 'none';

    if (view !== 'kanban' && view !== 'table' && view !== 'timeline') {
      this.advancedFilters = null;
    }

    await this.renderCurrentView();
  },

  async switchView(view) {
    await this._switchToView(view, true);
  },

  async renderCurrentView() {
    switch (this.currentView) {
      case 'dashboard': await Dashboard.render(); break;
      case 'kanban': await Kanban.render(); break;
      case 'table': await TableView.render(); break;
      case 'timeline': await Timeline.render(); break;
      case 'skills': await Skills.renderList(); break;
      case 'generated': await GeneratedContentView.render(); break;
      case 'settings': await Settings.render(); break;
      case 'job': await this.renderJobDetail(); break;
    }
  },

  async renderJobDetail() {
    const pane = document.getElementById('view-job');
    const jobId = this.currentJobId;
    if (!jobId) { await this.switchView('dashboard'); return; }

    const job = await DB.getJob(jobId);
    if (!job) { await this.switchView('dashboard'); return; }

    const history = await DB.getJobHistory(jobId);
    const titleEl = document.getElementById('view-title');
    titleEl.textContent = `${job.company} — ${job.position}`;

    pane.innerHTML = `
      <div class="job-detail-page">
        <button class="btn btn-sm btn-secondary job-back-btn" style="margin-bottom:16px">
          ${icon('arrow-left', 16)} Back
        </button>

        <div class="job-detail-header">
          <div>
            <h2 style="margin:0 0 4px">${UI.escapeHtml(job.company)}</h2>
            <h3 style="margin:0 0 12px;font-weight:400;color:var(--text-muted)">${UI.escapeHtml(job.position)}</h3>
          </div>
          <div>${UI.statusBadge(job.status)}</div>
        </div>

        <div class="job-detail-grid">
          <div class="detail-item"><span class="detail-label">Category</span><span>${UI.escapeHtml(job.category || 'General')}</span></div>
          <div class="detail-item"><span class="detail-label">Salary</span><span>${UI.formatCurrency(job.salary) || '-'}</span></div>
          <div class="detail-item"><span class="detail-label">Location</span><span>${UI.escapeHtml(job.location || '-')}</span></div>
          <div class="detail-item"><span class="detail-label">Contact</span><span>${UI.escapeHtml(job.contact || '-')}</span></div>
          <div class="detail-item"><span class="detail-label">Deadline</span><span>${UI.formatDate(job.date) || '-'}</span></div>
          <div class="detail-item"><span class="detail-label">Applied</span><span>${UI.formatDate(job.appliedDate) || '-'}</span></div>
          <div class="detail-item"><span class="detail-label">Created</span><span>${UI.formatDateTime(job.createdAt) || '-'}</span></div>
          <div class="detail-item"><span class="detail-label">Updated</span><span>${UI.formatDateTime(job.updatedAt) || '-'}</span></div>
        </div>

        ${job.url ? `<div class="job-detail-url"><span class="detail-label">URL</span> <a href="${UI.escapeHtml(job.url)}" target="_blank">${UI.escapeHtml(job.url)}</a></div>` : ''}

        ${job.notes ? `
          <div class="job-detail-section">
            <h4>Notes</h4>
            <div class="job-notes-content">${UI.renderMarkdown(job.notes)}</div>
          </div>
        ` : ''}

        <div class="job-detail-section">
          <h4>Activity History</h4>
          ${!history || history.length === 0 ? '<p class="text-muted text-sm">No history recorded yet.</p>' :
            history.map(h => `
              <div class="history-item">
                <div class="history-time">${UI.formatDateTime(h.timestamp)}</div>
                <div class="history-change">
                  ${h.action === 'Created' ? `${icon('plus', 14)} Job created` :
                    h.action === 'Status' ? `${icon('pin', 14)} Status: <span class="history-from">${UI.escapeHtml(h.from)}</span> → <span class="history-to">${UI.escapeHtml(h.to)}</span>` :
                    h.action === 'Deleted' ? `${icon('trash', 14)} Deleted` :
                    `${icon('edit', 14)} Updated`}
                </div>
              </div>
            `).join('')}
        </div>

        <div class="job-detail-section">
          <h4>CLI Quick Actions</h4>
          <pre style="background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:13px;line-height:1.6">
  waypoint update ${jobId} --status "Offer" --notes "New status"
  waypoint update ${jobId} --notes "Add a note here"
  waypoint delete ${jobId}</pre>
        </div>
      </div>
    `;

    pane.querySelector('.job-back-btn').addEventListener('click', () => {
      window.history.length > 1 ? window.history.back() : this.switchView('dashboard');
    });
  },

  async filterJobs() {
    const input = document.getElementById('search-input');
    this.searchQuery = input.value;
    await this.renderCurrentView();
  },

  async updateCounts() {
    await UI.renderCategories();
  },

  openJobForm() {
    UI.showToast('Use the CLI to manage jobs.', 'info');
  },

  saveJobForm() {
    UI.showToast('Use the CLI to manage jobs.', 'info');
  },

  showJobDetail(jobId) {
    this.currentJobId = jobId;
    this.switchView('job');
  },
};

document.addEventListener('DOMContentLoaded', async () => await App.init());
