/* === Main Application (read-only, URL-routed) === */
const App = {
  currentView: null,
  currentCategory: 'all',
  currentJobId: null,
  currentArtifactId: null,
  searchQuery: '',
  advancedFilters: null,
  tableCategoryFilter: null,

  _views: ['dashboard', 'kanban', 'table', 'timeline', 'categories', 'profile', 'skills', 'artifacts', 'settings', 'job', 'artifact'],

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
    const artifactMatch = path.match(/^artifact\/(\d+)$/);
    if (artifactMatch) {
      this._pendingArtifactId = parseInt(artifactMatch[1], 10);
      return 'artifact';
    }
    if (this._views.includes(path)) return path;
    return null;
  },

  async _switchToView(view, pushHistory) {
    if (!view) return;

    // Allow re-entering 'job'/'artifact' view for different IDs
    if (view !== 'job' && view !== 'artifact' && this.currentView === view) return;

    const prevView = this.currentView;
    this.currentView = view;

    // Clear breadcrumbs for list views (detail views set their own)
    if (view !== 'job' && view !== 'artifact') {
      this.setBreadcrumb(null);
    }

    // Resolve pending IDs from URL
    if (view === 'job' && this._pendingJobId) {
      this.currentJobId = this._pendingJobId;
      this._pendingJobId = null;
    }
    if (view === 'artifact' && this._pendingArtifactId) {
      this.currentArtifactId = this._pendingArtifactId;
      this._pendingArtifactId = null;
    }

    if (pushHistory) {
      let path;
      if (view === 'dashboard') path = '/';
      else if (view === 'job') path = '/job/' + this.currentJobId;
      else if (view === 'artifact') path = '/artifact/' + this.currentArtifactId;
      else path = '/' + view;
      history.pushState({ view }, '', path);
    }

    // Title
    const titles = {
      dashboard: 'Dashboard', kanban: 'Kanban Board', table: 'Table View',
      timeline: 'Timeline', categories: 'Categories', profile: 'Profile', skills: 'AI Integration', artifacts: 'Artifacts',
      settings: 'Settings', job: this.currentJobId ? 'Job #' + this.currentJobId : 'Job Detail',
      artifact: this.currentArtifactId ? 'Artifact #' + this.currentArtifactId : 'Artifact',
    };
    document.title = (titles[view] || 'Dashboard') + ' — Waypoint';

    // View panes
    document.querySelectorAll('.view-pane').forEach(p => p.classList.remove('active'));
    let pane = document.getElementById('view-' + view);
    if (!pane && (view === 'job' || view === 'artifact')) {
      pane = document.createElement('div');
      pane.className = 'view-pane';
      pane.id = 'view-' + view;
      document.getElementById('view-container').appendChild(pane);
    }
    if (pane) pane.classList.add('active');

    // Nav items (skip for job detail — no active nav)
    if (view !== 'job' && view !== 'artifact') {
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

  // Breadcrumb management
  setBreadcrumb(items) {
    const bar = document.getElementById('breadcrumb-bar');
    const title = document.getElementById('view-title');
    if (!items || items.length === 0) {
      // Simple title mode — no breadcrumbs
      bar.classList.remove('breadcrumb-mode');
      return;
    }
    bar.classList.add('breadcrumb-mode');
    const existing = bar.querySelector('.breadcrumb');
    if (existing) existing.remove();

    const nav = document.createElement('nav');
    nav.className = 'breadcrumb';
    items.forEach((item, i) => {
      if (i > 0) {
        const sep = document.createElement('span');
        sep.className = 'breadcrumb-sep';
        sep.innerHTML = icon('chevron-r', 12);
        nav.appendChild(sep);
      }
      if (i < items.length - 1) {
        const a = document.createElement('a');
        a.href = item.href || '#';
        a.className = 'breadcrumb-link';
        a.textContent = item.label;
        if (item.action) {
          a.addEventListener('click', e => { e.preventDefault(); item.action(); });
        }
        nav.appendChild(a);
      } else {
        const span = document.createElement('span');
        span.className = 'breadcrumb-current';
        span.textContent = item.label;
        nav.appendChild(span);
      }
    });
    bar.appendChild(nav);
  },

  async renderCurrentView() {
    switch (this.currentView) {
      case 'dashboard': await Dashboard.render(); break;
      case 'kanban': await Kanban.render(); break;
      case 'table': await TableView.render(); break;
      case 'timeline': await Timeline.render(); break;
      case 'categories': await CategoriesView.render(); break;
      case 'profile': await ProfileView.render(); break;
      case 'skills': await Skills.renderList(); break;
      case 'artifacts': await GeneratedContentView.render(); break;
      case 'settings': await Settings.render(); break;
      case 'artifact': await this.renderArtifactDetail(); break;
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

    this.setBreadcrumb([
      { label: 'Jobs', action: () => this.switchView('table') },
      { label: `${job.company} — ${job.position}` },
    ]);

    pane.innerHTML = `
      <div class="job-detail-page">

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

        <div class="job-detail-section" id="job-artifacts-section">
          <h4>${icon('folder', 16)} Artifacts</h4>
          <p class="text-muted text-sm">Loading...</p>
        </div>

        <div class="job-detail-section">
          <h4>CLI Quick Actions</h4>
          <pre style="background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:13px;line-height:1.6">
  waypoint jobs update ${jobId} --status "Offer" --notes "New status"
  waypoint jobs update ${jobId} --notes "Add a note here"
  waypoint jobs delete ${jobId}</pre>
        </div>
      </div>
    `;


    // Load linked artifacts
    this._renderJobArtifacts(jobId);
  },

  async _renderJobArtifacts(jobId) {
    const section = document.getElementById('job-artifacts-section');
    if (!section) return;

    const artifacts = await DB.getArtifacts(null, jobId);
    const skillLabels = {
      'email-generator': 'Email',
      'cover-letter': 'Cover Letter',
      'resume-optimizer': 'Resume Optimizer',
      'interview-prep': 'Interview Prep',
      'career-summary': 'Career Summary',
    };

    if (artifacts.length === 0) {
      section.innerHTML = `<h4>${icon('folder', 16)} Artifacts</h4><p class="text-muted text-sm">No artifacts generated for this job yet.</p>`;
      return;
    }

    section.innerHTML = `
      <h4>${icon('folder', 16)} Artifacts (${artifacts.length})</h4>
      <div class="job-artifact-list">
        ${artifacts.map(a => {
          let variants = [];
          try { variants = JSON.parse(a.variants || '[]'); } catch { variants = []; }
          return `
            <div class="job-artifact-item" data-artifact-id="${a.id}" style="cursor:pointer">
              <span class="gen-skill-badge">${skillLabels[a.skillId] || a.skillId}</span>
              <span class="job-artifact-title">${UI.escapeHtml(a.title)}</span>
              <span class="job-artifact-variants">${variants.length} variant${variants.length === 1 ? '' : 's'}</span>
              <span class="job-artifact-date">${UI.formatDate(a.createdAt)}</span>
            </div>
          `;
        }).join('')}
      </div>
    `;

    section.querySelectorAll('.job-artifact-item').forEach(el => {
      el.addEventListener('click', () => {
        this.showArtifactDetail(parseInt(el.dataset.artifactId));
      });
    });
  },

  async filterJobs() {
    const input = document.getElementById('search-input');
    this.searchQuery = input.value;
    await this.renderCurrentView();
  },

  async updateCounts() {
    // Refresh handled by individual views
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

  showArtifactDetail(artifactId) {
    this.currentArtifactId = artifactId;
    this.switchView('artifact');
  },

  async renderArtifactDetail() {
    const pane = document.getElementById('view-artifact');
    const artifactId = this.currentArtifactId;
    if (!artifactId) { await this.switchView('artifacts'); return; }

    const art = await DB.getArtifact(artifactId);
    if (!art) { await this.switchView('artifacts'); return; }

    const titleEl = document.getElementById('view-title');
    titleEl.textContent = art.title || 'Artifact';

    // Build breadcrumb trail
    const crumbs = [{ label: 'Artifacts', action: () => this.switchView('artifacts') }];
    if (art.jobId) {
      const job = await DB.getJob(art.jobId);
      if (job) {
        crumbs.push({ label: job.company, action: () => this.showJobDetail(job.id) });
      }
    }
    crumbs.push({ label: art.title || 'Artifact' });
    this.setBreadcrumb(crumbs);

    let variants = [];
    try { variants = JSON.parse(art.variants || '[]'); } catch { variants = []; }

    // Resolve job info
    let jobLink = '';
    if (art.jobId) {
      const job = await DB.getJob(art.jobId);
      if (job) {
        jobLink = `<a href="#" class="gen-job-link" data-job-id="${job.id}">${UI.escapeHtml(job.company)} — ${UI.escapeHtml(job.position)}</a>`;
      }
    }

    const skillLabels = {
      'email-generator': 'Email',
      'cover-letter': 'Cover Letter',
      'resume-optimizer': 'Resume Optimizer',
      'interview-prep': 'Interview Prep',
      'career-summary': 'Career Summary',
    };

    pane.innerHTML = `
      <div class="artifact-detail-page">

        <div class="artifact-detail-header">
          <div>
            <h2 style="margin:0 0 4px">${UI.escapeHtml(art.title || 'Untitled')}</h2>
            <div style="font-size:13px;color:var(--text-muted);display:flex;align-items:center;gap:8px;flex-wrap:wrap">
              <span class="gen-skill-badge">${skillLabels[art.skillId] || art.skillId}</span>
              ${jobLink ? ' · ' + jobLink : ''}
              · ${UI.formatDateTime(art.createdAt)}
            </div>
          </div>
        </div>

        ${variants.length > 1 ? `
          <div class="artifact-variant-tabs">
            ${variants.map((v, i) => `<button class="artifact-variant-tab${i === 0 ? ' active' : ''}" data-variant="${i}">${UI.escapeHtml(v.label || 'Variant ' + (i+1))}</button>`).join('')}
          </div>
        ` : ''}

        <div class="artifact-variant-panes">
          ${variants.map((v, i) => `
            <div class="artifact-variant-pane${i === 0 ? '' : ' hidden'}" data-variant-pane="${i}">
              <div class="artifact-variant-content">${UI.escapeHtml(v.content || '')}</div>
            </div>
          `).join('')}
        </div>

        <div class="artifact-detail-section">
          <h4>${icon('terminal', 16)} CLI</h4>
          <pre style="background:var(--bg-secondary);padding:12px;border-radius:6px;font-size:13px;line-height:1.6">waypoint artifacts get ${artifactId}
waypoint artifacts archive ${artifactId}
waypoint artifacts delete ${artifactId}</pre>
        </div>
      </div>
    `;


    // Job link
    pane.querySelectorAll('.gen-job-link').forEach(a => {
      a.addEventListener('click', e => {
        e.preventDefault();
        App.showJobDetail(parseInt(a.dataset.jobId));
      });
    });

    // Variant tabs
    pane.querySelectorAll('.artifact-variant-tab').forEach(tab => {
      tab.addEventListener('click', () => {
        const idx = tab.dataset.variant;
        pane.querySelectorAll('.artifact-variant-tab').forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        pane.querySelectorAll('.artifact-variant-pane').forEach(p => p.classList.add('hidden'));
        pane.querySelector(`[data-variant-pane="${idx}"]`).classList.remove('hidden');
      });
    });
  },
};

document.addEventListener('DOMContentLoaded', async () => await App.init());
