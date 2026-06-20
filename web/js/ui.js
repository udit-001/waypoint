/* === UI Utilities === */
const UI = {
  init() {
    this.initSidebar();
    this.initModals();
    this.initThemeToggle();
    this.initViewToggles();
    this.initExportImport();
    this.initSearch();
    this.initKeyboardShortcuts();
    this.addReadOnlyStyles();
  },

  initSidebar() {
    const STORAGE_KEY = 'jobtracker_sidebar_closed';
    const toggleBtn = document.getElementById('sidebar-toggle');
    const sidebar = document.getElementById('sidebar');
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'true') {
      sidebar.classList.add('closed');
    } else {
      sidebar.classList.remove('closed');
    }
    toggleBtn.addEventListener('click', () => {
      sidebar.classList.toggle('closed');
      localStorage.setItem(STORAGE_KEY, sidebar.classList.contains('closed'));
    });

    document.querySelectorAll('[data-view]').forEach(el => {
      el.addEventListener('click', async e => {
        e.preventDefault();
        const view = el.dataset.view;
        await App.switchView(view);
        document.querySelectorAll('.nav-item[data-view]').forEach(n => n.classList.remove('active'));
        el.classList.add('active');
      });
    });

  },

  async renderCategories() {
    // Removed — categories now have their own page
  },

  initModals() {
    document.querySelectorAll('.modal-close').forEach(btn => {
      btn.addEventListener('click', () => {
        const modal = document.getElementById(btn.dataset.modal);
        if (modal) modal.classList.remove('active');
      });
    });
    document.querySelectorAll('.modal-overlay').forEach(overlay => {
      overlay.addEventListener('click', e => {
        if (e.target === overlay) overlay.classList.remove('active');
      });
    });
    document.querySelectorAll('[data-modal]').forEach(btn => {
      if (btn.classList.contains('modal-close')) return;
      btn.addEventListener('click', () => {
        const modal = document.getElementById(btn.dataset.modal);
        if (modal) modal.classList.add('active');
      });
    });
  },

  initThemeToggle() {
    const btn = document.getElementById('theme-toggle');
    btn.addEventListener('click', () => {
      const html = document.documentElement;
      const current = html.dataset.theme;
      const next = current === 'light' ? 'dark' : 'light';
      html.dataset.theme = next;
      btn.innerHTML = icon(next === 'light' ? 'moon' : 'sun', 18);
      localStorage.setItem('jobtracker_theme', next);
    });
  },

  initViewToggles() {
    document.querySelectorAll('.view-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        await App.switchView(btn.dataset.view);
      });
    });
  },

  initExportImport() {
    // Read-only — disabled
  },

  initSearch() {
    Search.init();
  },

  initKeyboardShortcuts() {
    document.addEventListener('keydown', e => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
      if (e.ctrlKey && e.key === 'n') { e.preventDefault(); UI.showToast('Use the CLI to add jobs', 'info'); }
      if (e.ctrlKey && e.key === 'f') {
        e.preventDefault();
        const input = document.getElementById('search-input');
        input.focus();
        input.select();
      }
    });
  },

  async showAdvancedSearch() {
    const cats = await DB.getCategories();
    const statuses = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];
    const body = document.createElement('div');
    body.innerHTML = `
      <div class="advanced-search">
        <div class="form-group"><label>Company</label><input id="adv-company" placeholder="Search company..." /></div>
        <div class="form-group"><label>Position</label><input id="adv-position" placeholder="Search position..." /></div>
        <div class="form-group"><label>Category</label><select id="adv-category"><option value="">Any</option>${cats.map(c => `<option value="${c.name}">${c.name}</option>`).join('')}</select></div>
        <div class="form-group"><label>Status</label><select id="adv-status"><option value="">Any</option>${statuses.map(s => `<option value="${s}">${s}</option>`).join('')}</select></div>
        <div class="form-group"><label>Date From</label><input type="date" id="adv-date-from" /></div>
        <div class="form-group"><label>Date To</label><input type="date" id="adv-date-to" /></div>
        <div class="form-group" style="grid-column:1/-1"><label>Notes contain</label><input id="adv-notes" placeholder="Search in notes..." /></div>
      </div>
    `;
    const modal = document.getElementById('skills-modal');
    document.getElementById('skills-modal-title').textContent = 'Advanced Search';
    const bodyEl = document.getElementById('skills-modal-body');
    bodyEl.innerHTML = '';
    bodyEl.appendChild(body);
    const footer = modal.querySelector('.modal-footer');
    footer.innerHTML = '<button class="btn btn-secondary" data-modal="skills-modal">Cancel</button><button class="btn btn-primary" id="adv-search-btn">Search</button>';
    modal.classList.add('active');
    document.getElementById('adv-search-btn').addEventListener('click', async () => {
      const filters = {
        company: document.getElementById('adv-company').value.toLowerCase(),
        position: document.getElementById('adv-position').value.toLowerCase(),
        category: document.getElementById('adv-category').value,
        status: document.getElementById('adv-status').value,
        dateFrom: document.getElementById('adv-date-from').value,
        dateTo: document.getElementById('adv-date-to').value,
        notes: document.getElementById('adv-notes').value.toLowerCase(),
      };
      App.advancedFilters = filters;

      // If there's text in company, position, or notes, build an FTS query
      // to pre-filter server-side, then apply remaining filters client-side
      const ftsTerms = [];
      if (filters.company) ftsTerms.push(filters.company);
      if (filters.position) ftsTerms.push(filters.position);
      if (filters.notes) ftsTerms.push(filters.notes);

      if (ftsTerms.length > 0) {
        // Use FTS for the broad search, then refine client-side
        const ftsQuery = ftsTerms.join(' OR ');
        App.searchQuery = ftsQuery;
        // Clear the specific FTS fields so they don't double-filter client-side
        // (FTS already matched them)
        App._advancedFtsFields = { company: filters.company, position: filters.position, notes: filters.notes };
        filters.company = '';
        filters.position = '';
        filters.notes = '';
      }

      modal.classList.remove('active');
      await App.switchView('table');
    });
  },

  showToast(msg, type = 'info') {
    const toast = document.createElement('div');
    toast.style.cssText = `position:fixed;bottom:80px;right:24px;padding:10px 18px;border-radius:8px;font-size:13px;font-weight:500;z-index:300;animation:modalIn 200ms ease;background:${type === 'error' ? 'var(--danger)' : type === 'success' ? 'var(--success)' : 'var(--accent)'};color:#fff;box-shadow:var(--shadow-md);`;
    toast.textContent = msg;
    document.body.appendChild(toast);
    setTimeout(() => { toast.style.opacity = '0'; toast.style.transition = 'opacity 300ms'; setTimeout(() => toast.remove(), 300); }, 2500);
  },

  confirmDialog(msg) {
    return new Promise(resolve => {
      const modal = document.getElementById('skills-modal');
      document.getElementById('skills-modal-title').textContent = 'Confirm';
      document.getElementById('skills-modal-body').innerHTML = `<p style="font-size:14px">${msg}</p>`;
      const footer = modal.querySelector('.modal-footer');
      footer.innerHTML = '<button class="btn btn-secondary" id="confirm-no">Cancel</button><button class="btn btn-primary" id="confirm-yes">Confirm</button>';
      modal.classList.add('active');
      document.getElementById('confirm-yes').onclick = () => { modal.classList.remove('active'); resolve(true); };
      document.getElementById('confirm-no').onclick = () => { modal.classList.remove('active'); resolve(false); };
    });
  },

  showEmptyState(container, iconHtml, title, desc) {
    container.innerHTML = `<div class="empty-state"><div class="empty-icon">${iconHtml}</div><h3>${title}</h3><p>${desc}</p></div>`;
  },

  addReadOnlyStyles() {
    const style = document.createElement('style');
    style.textContent = `
      .read-only-badge {
        background: var(--bg-secondary);
        border: 1px solid var(--border);
        border-radius: 6px;
        padding: 4px 10px;
        font-size: 11px;
        color: var(--text-muted);
        position: fixed;
        bottom: 16px;
        left: 50%;
        transform: translateX(-50%);
        z-index: 200;
        opacity: 0.8;
        pointer-events: none;
        white-space: nowrap;
      }
    `;
    document.head.appendChild(style);

    // Add a small indicator
    const badge = document.createElement('div');
    badge.className = 'read-only-badge';
    badge.textContent = '🔒 Read-only — manage via CLI';
    document.body.appendChild(badge);
  },

  // === Helper utilities ===

  escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  },

  statusBadge(status) {
    const colors = {
      'Not Applied': '#81A1C1',
      'Applied': '#5E81AC',
      'Offer': '#A3BE8C',
      'Rejected': '#BF616A',
      'Withdrawn': '#4C566A',
    };
    return `<span style="display:inline-block;padding:2px 8px;border-radius:4px;font-size:12px;font-weight:500;color:#fff;background:${colors[status] || '#888'}">${this.escapeHtml(status)}</span>`;
  },

  formatDate(d) {
    if (!d) return '';
    try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
    catch { return d; }
  },

  formatDateTime(d) {
    if (!d) return '';
    try { return new Date(d).toLocaleString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' }); }
    catch { return d; }
  },

  formatCurrency(s) {
    return s || '';
  },

  renderMarkdown(text) {
    if (!text) return '';
    if (typeof marked !== 'undefined') {
      try { return marked.parse(text); } catch { return text; }
    }
    return text.replace(/\n/g, '<br>');
  },
};
