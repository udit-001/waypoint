/* === UI Utilities === */
const UI = {
  init() {
    this.initSidebar();
    this.initModals();
    this.initThemeToggle();
    this.initViewToggles();
    this.initFAB();
    this.initExportImport();
    this.initSearch();
    this.initKeyboardShortcuts();
  },

  initSidebar() {
    // Toggle sidebar open/closed, persisted in localStorage
    const STORAGE_KEY = 'jobtracker_sidebar_closed';
    const toggleBtn = document.getElementById('sidebar-toggle');
    const sidebar = document.getElementById('sidebar');
    // Restore saved state; default to closed if not set
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'false') {
      sidebar.classList.remove('closed');
    } else {
      sidebar.classList.add('closed');
    }
    toggleBtn.addEventListener('click', () => {
      sidebar.classList.toggle('closed');
      localStorage.setItem(STORAGE_KEY, sidebar.classList.contains('closed'));
    });

    document.querySelectorAll('[data-view]').forEach(el => {
      el.addEventListener('click', e => {
        e.preventDefault();
        const view = el.dataset.view;
        App.switchView(view);
        document.querySelectorAll('.nav-item[data-view]').forEach(n => n.classList.remove('active'));
        el.classList.add('active');
      });
    });
    document.getElementById('add-category-btn').addEventListener('click', () => {
      const name = prompt('Enter category name:');
      if (name && name.trim()) {
        DB.addCategory(name.trim());
        this.renderCategories();
        App.renderCurrentView();
      }
    });
  },

  renderCategories() {
    const list = document.getElementById('category-list');
    const cats = DB.getCategories();
    list.innerHTML = '';

    // All Jobs link — created as an element so it keeps its click handler
    const allLink = document.createElement('a');
    allLink.className = 'nav-item' + (App.currentCategory === 'all' ? ' active' : '');
    allLink.dataset.category = 'all';
    allLink.href = '#';
    allLink.textContent = 'All Jobs';
    allLink.addEventListener('click', e => {
      e.preventDefault();
      App.currentCategory = 'all';
      document.querySelectorAll('[data-category]').forEach(n => n.classList.remove('active'));
      allLink.classList.add('active');
      App.renderCurrentView();
    });
    list.appendChild(allLink);
    cats.forEach(c => {
      const wrapper = document.createElement('div');
      wrapper.className = 'nav-item' + (App.currentCategory === c ? ' active' : '');
      wrapper.dataset.category = c;
      wrapper.style.cssText = 'display:flex;align-items:center;padding:0';
      wrapper.addEventListener('click', e => {
        if (e.target.tagName === 'BUTTON') return;
        e.preventDefault();
        App.currentCategory = c;
        document.querySelectorAll('[data-category]').forEach(n => n.classList.remove('active'));
        wrapper.classList.add('active');
        App.renderCurrentView();
      });

      const a = document.createElement('a');
      a.href = '#';
      a.textContent = c;
      a.style.cssText = 'flex:1;padding:6px 12px;color:inherit;text-decoration:none;display:block';

      const del = document.createElement('button');
      del.textContent = '×';
      del.title = 'Delete category';
      del.style.cssText = 'background:none;border:none;color:var(--text-muted);cursor:pointer;padding:4px 8px;font-size:16px;line-height:1;display:none';
      del.addEventListener('click', async e => {
        e.stopPropagation();
        const confirmed = await UI.confirmDialog(`Delete category "${c}"? Jobs in this category will be moved to "General".`);
        if (confirmed) {
          DB.deleteCategory(c);
          if (App.currentCategory === c) { App.currentCategory = 'all'; }
          this.renderCategories();
          App.renderCurrentView();
        }
      });

      wrapper.addEventListener('mouseenter', () => { del.style.display = 'block'; });
      wrapper.addEventListener('mouseleave', () => { del.style.display = 'none'; });

      wrapper.appendChild(a);
      wrapper.appendChild(del);
      list.appendChild(wrapper);
    });
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
      const settings = DB.getSettings();
      settings.theme = next;
      DB.saveSettings(settings);
    });
  },

  initViewToggles() {
    document.querySelectorAll('.view-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const view = btn.dataset.view;
        App.switchView(view);
      });
    });
  },

  initFAB() {
    document.getElementById('fab').addEventListener('click', () => {
      App.openJobForm();
    });
  },

  initExportImport() {
    const expBtn = document.getElementById('export-btn');
    const impBtn = document.getElementById('import-btn');
    if (expBtn) expBtn.addEventListener('click', Export.exportData);
    if (impBtn) impBtn.addEventListener('click', () => {
      document.getElementById('import-file').click();
    });
    document.getElementById('import-file').addEventListener('change', Export.importData);
  },

  initSearch() {
    document.getElementById('search-input').addEventListener('input', App.filterJobs);
    document.getElementById('search-btn').addEventListener('click', () => {
      UI.showAdvancedSearch();
    });
  },

  initKeyboardShortcuts() {
    document.addEventListener('keydown', e => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
      if (e.ctrlKey && e.key === 'n') { e.preventDefault(); App.openJobForm(); }
      if (e.ctrlKey && e.key === 'f') { e.preventDefault(); document.getElementById('search-input').focus(); }
      if (e.ctrlKey && e.key === 's') { e.preventDefault(); Export.exportData(); }
    });
  },

  showAdvancedSearch() {
    const cats = DB.getCategories();
    const statuses = ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'];
    const body = document.createElement('div');
    body.innerHTML = `
      <div class="advanced-search">
        <div class="form-group"><label>Company</label><input id="adv-company" placeholder="Search company..." /></div>
        <div class="form-group"><label>Position</label><input id="adv-position" placeholder="Search position..." /></div>
        <div class="form-group"><label>Category</label><select id="adv-category"><option value="">Any</option>${cats.map(c => `<option value="${c}">${c}</option>`).join('')}</select></div>
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
    document.getElementById('adv-search-btn').addEventListener('click', () => {
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
      modal.classList.remove('active');
      App.renderCurrentView();
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

  showEmptyState(container, icon, title, desc) {
    container.innerHTML = `<div class="empty-state"><div class="empty-icon">${icon}</div><h3>${title}</h3><p>${desc}</p></div>`;
  },

  statusBadge(status) {
    const cls = 'status-' + status.replace(/\s+/g, '\\ ');
    return `<span class="status-badge ${cls}">${status}</span>`;
  },

  formatDate(d) {
    if (!d) return '';
    const date = new Date(d);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  },

  formatDateTime(d) {
    if (!d) return '';
    const date = new Date(d);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit' });
  },

  formatCurrency(s) {
    return s || '-';
  },

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  },

  renderMarkdown(text) {
    if (!text) return '';
    if (typeof marked !== 'undefined') {
      return marked.parse(text);
    }
    return text.replace(/\n/g, '<br>');
  },
};
