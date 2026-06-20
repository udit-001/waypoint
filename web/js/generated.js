/* === Generated Content / Artifacts View === */
const GeneratedContentView = {
  activeSkillFilter: null,

  async render() {
    const container = document.getElementById('generated-container');
    const items = await DB.getArtifacts(this.activeSkillFilter);

    // Fetch jobs to resolve job links
    const jobs = await DB.getJobs();
    const jobMap = {};
    jobs.forEach(j => { jobMap[j.id] = j; });

    document.getElementById('view-title').textContent = 'Artifacts';

    // Skill filter chips
    const skills = [
      { id: null, name: 'All' },
      { id: 'email-generator', name: 'Emails' },
      { id: 'cover-letter', name: 'Cover Letters' },
      { id: 'resume-optimizer', name: 'Resume Optimizer' },
      { id: 'interview-prep', name: 'Interview Prep' },
      { id: 'career-summary', name: 'Career Summary' },
    ];

    const filterBar = skills.map(s => `
      <button class="btn btn-sm ${this.activeSkillFilter === s.id ? 'btn-primary' : 'btn-secondary'}" data-skill="${s.id === null ? '' : s.id}">${s.name}</button>
    `).join('');

    if (items.length === 0) {
      container.innerHTML = `<div style="margin-bottom:16px">${filterBar}</div>`;
      UI.showEmptyState(container, icon('folder', 48), 'No artifacts yet', 'Generate content via the CLI skills. Artifacts store every variant (tones, lengths, styles).');
      this._bindFilters(container);
      return;
    }

    container.innerHTML = `
      <div style="margin-bottom:16px">${filterBar}</div>
      ${items.map(item => this._renderItem(item, jobMap)).join('')}
    `;

    this._bindFilters(container);
    this._bindItemActions(container);
  },

  _renderItem(item, jobMap) {
    let variants = [];
    try { variants = JSON.parse(item.variants || '[]'); } catch { variants = []; }

    const skillLabel = this._skillLabel(item.skillId);

    // Job link
    let jobLink = '';
    if (item.jobId && jobMap[item.jobId]) {
      const j = jobMap[item.jobId];
      jobLink = `<a href="#" class="gen-job-link" data-job-id="${j.id}">${UI.escapeHtml(j.company)} — ${UI.escapeHtml(j.position)}</a>`;
    }

    const variantTabs = variants.length > 1
      ? `<div class="gen-variant-tabs">${variants.map((v, i) => `<button class="gen-variant-tab${i === 0 ? ' active' : ''}" data-variant="${i}">${UI.escapeHtml(v.label || 'Variant ' + (i+1))}</button>`).join('')}</div>`
      : '';

    return `
      <div class="generated-item" data-id="${item.id}">
        <div class="gen-header">
          <div>
            <div class="gen-title">${UI.escapeHtml(item.title || 'Untitled')}</div>
            <div class="gen-meta">
              <span class="gen-skill-badge">${skillLabel}</span>
              ${jobLink ? ' · ' + jobLink : ''}
              · ${variants.length} variant${variants.length === 1 ? '' : 's'}
              · ${UI.formatDateTime(item.createdAt)}
            </div>
          </div>
          <div class="gen-actions">
            <button class="btn btn-sm btn-secondary copy-gen-btn" data-id="${item.id}">${icon('copy', 14)} Copy</button>
            <button class="btn btn-sm btn-secondary get-gen-btn" data-id="${item.id}">${icon('eye', 14)}</button>
          </div>
        </div>
        ${variantTabs}
        <div class="gen-variant-panes">
          ${variants.map((v, i) => `
            <div class="gen-variant-pane${i === 0 ? '' : ' hidden'}" data-variant-pane="${i}">
              <div class="gen-content">${UI.escapeHtml(v.content || '')}</div>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  },

  _skillLabel(id) {
    const labels = {
      'email-generator': 'Email',
      'cover-letter': 'Cover Letter',
      'resume-optimizer': 'Resume Optimizer',
      'interview-prep': 'Interview Prep',
      'career-summary': 'Career Summary',
    };
    return labels[id] || id || 'Unknown';
  },

  _bindFilters(container) {
    container.querySelectorAll('[data-skill]').forEach(btn => {
      btn.addEventListener('click', async () => {
        this.activeSkillFilter = btn.dataset.skill || null;
        await this.render();
      });
    });
  },

  _bindItemActions(container) {
    // Job links → navigate to job detail
    container.querySelectorAll('.gen-job-link').forEach(a => {
      a.addEventListener('click', e => {
        e.preventDefault();
        App.showJobDetail(parseInt(a.dataset.jobId));
      });
    });

    // Variant tab switching
    container.querySelectorAll('.gen-variant-tab').forEach(tab => {
      tab.addEventListener('click', () => {
        const item = tab.closest('.generated-item');
        const idx = tab.dataset.variant;
        item.querySelectorAll('.gen-variant-tab').forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        item.querySelectorAll('.gen-variant-pane').forEach(p => p.classList.add('hidden'));
        item.querySelector(`[data-variant-pane="${idx}"]`).classList.remove('hidden');
      });
    });

    // Copy first non-hidden variant
    container.querySelectorAll('.copy-gen-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const item = btn.closest('.generated-item');
        const activePane = item.querySelector('.gen-variant-pane:not(.hidden)');
        const content = activePane ? activePane.querySelector('.gen-content').textContent : '';
        await navigator.clipboard.writeText(content);
        UI.showToast('Copied to clipboard!', 'success');
      });
    });

    // View full artifact in modal
    container.querySelectorAll('.get-gen-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const id = btn.dataset.id;
        const art = await DB.getArtifact(id);
        this._showModal(art);
      });
    });
  },

  _showModal(art) {
    let variants = [];
    try { variants = JSON.parse(art.variants || '[]'); } catch { variants = []; }

    const modal = document.getElementById('skills-modal');
    document.getElementById('skills-modal-title').textContent = art.title || 'Artifact';
    const body = document.getElementById('skills-modal-body');
    body.innerHTML = `
      <div class="gen-modal">
        <div class="gen-modal-meta">
          <span class="gen-skill-badge">${this._skillLabel(art.skillId)}</span>
          · ${UI.formatDateTime(art.createdAt)}
        </div>
        ${variants.map((v, i) => `
          <div class="gen-modal-variant">
            <h4>${UI.escapeHtml(v.label || 'Variant ' + (i+1))}</h4>
            <pre class="gen-modal-content">${UI.escapeHtml(v.content || '')}</pre>
        </div>
        `).join('')}
      </div>
    `;
    const footer = modal.querySelector('.modal-footer');
    footer.innerHTML = '<button class="btn btn-secondary" data-modal="skills-modal">Close</button>';
    modal.querySelector('[data-modal]').addEventListener('click', () => modal.classList.remove('active'));
    modal.classList.add('active');
  },
};
