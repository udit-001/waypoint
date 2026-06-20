/* === Full-Text Search Module (server-side FTS5) === */
const Search = {
  _debounceTimer: null,
  _debounceMs: 250,
  _dropdownVisible: false,
  _lastQuery: '',

  init() {
    const input = document.getElementById('search-input');
    const btn = document.getElementById('search-btn');
    const clearBtn = document.getElementById('search-clear');

    if (!input) return;

    // Debounced input handler
    input.addEventListener('input', () => {
      this._showClearButton();
      clearTimeout(this._debounceTimer);
      const q = input.value.trim();
      if (q.length < 2) {
        this._hideDropdown();
        App.searchQuery = '';
        App.advancedFilters = null;
        this._debounceTimer = setTimeout(() => App.renderCurrentView(), 150);
        return;
      }
      this._debounceTimer = setTimeout(async () => {
        App.searchQuery = q;
        App.advancedFilters = null;
        await App.renderCurrentView();
        // Show global dropdown if we're not on a list view
        const view = App.currentView;
        if (view !== 'kanban' && view !== 'table') {
          await this._showGlobalResults(q);
        } else {
          this._hideDropdown();
        }
      }, this._debounceMs);
    });

    // Enter key — navigate to first result or show all
    input.addEventListener('keydown', async (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        const q = input.value.trim();
        if (q.length < 2) return;

        // Navigate to table view with the search applied
        App.searchQuery = q;
        App.advancedFilters = null;
        this._hideDropdown();
        await App.switchView('table');
      } else if (e.key === 'Escape') {
        this._hideDropdown();
        input.blur();
      }
    });

    // Clear button
    if (clearBtn) {
      clearBtn.addEventListener('click', async () => {
        input.value = '';
        App.searchQuery = '';
        App.advancedFilters = null;
        this._hideClearButton();
        this._hideDropdown();
        await App.renderCurrentView();
        input.focus();
      });
    }

    // Advanced search button
    if (btn) {
      btn.addEventListener('click', () => {
        UI.showAdvancedSearch();
      });
    }

    // Close dropdown on outside click
    document.addEventListener('click', (e) => {
      if (this._dropdownVisible && !e.target.closest('.search-box') && !e.target.closest('#search-dropdown')) {
        this._hideDropdown();
      }
    });

    this._showClearButton();
  },

  _showClearButton() {
    const btn = document.getElementById('search-clear');
    const input = document.getElementById('search-input');
    if (btn) btn.style.display = input && input.value.length > 0 ? 'flex' : 'none';
  },

  _hideClearButton() {
    const btn = document.getElementById('search-clear');
    if (btn) btn.style.display = 'none';
  },

  async _showGlobalResults(query) {
    if (query === this._lastQuery && this._dropdownVisible) return;
    this._lastQuery = query;

    const results = await DB.searchAll(query);
    if (results.length === 0) {
      this._showDropdown(this._emptyState(query));
      return;
    }

    const jobs = results.filter(r => r.type === 'job');
    const artifacts = results.filter(r => r.type === 'artifact');

    let html = '';

    if (jobs.length > 0) {
      html += `<div class="search-group"><div class="search-group-label">Jobs</div>`;
      jobs.forEach(j => {
        html += `<div class="search-result" data-type="job" data-id="${j.id}">
          <span class="search-result-icon">${icon('briefcase', 14)}</span>
          <span class="search-result-title">${this._highlight(UI.escapeHtml(j.title), query)}</span>
          <span class="search-result-sub">${UI.escapeHtml(j.sub)}</span>
        </div>`;
      });
      html += `</div>`;
    }

    if (artifacts.length > 0) {
      html += `<div class="search-group"><div class="search-group-label">Artifacts</div>`;
      artifacts.forEach(a => {
        const skillLabels = {
          'email-generator': 'Email',
          'cover-letter': 'Cover Letter',
          'resume-optimizer': 'Resume Optimizer',
          'interview-prep': 'Interview Prep',
          'career-summary': 'Career Summary',
          'statement-of-purpose': 'SOP',
        };
        html += `<div class="search-result" data-type="artifact" data-id="${a.id}">
          <span class="search-result-icon">${icon('folder', 14)}</span>
          <span class="search-result-title">${this._highlight(UI.escapeHtml(a.title || 'Untitled'), query)}</span>
          <span class="search-result-sub">${UI.escapeHtml(skillLabels[a.sub] || a.sub)}</span>
        </div>`;
      });
      html += `</div>`;
    }

    html += `<div class="search-footer">Press Enter for full results</div>`;

    this._showDropdown(html);
    this._bindResultClicks();
  },

  _emptyState(query) {
    return `<div class="search-empty">No results for "${UI.escapeHtml(query)}"</div>`;
  },

  _highlight(text, query) {
    if (!query) return text;
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(`(${escaped})`, 'gi');
    return text.replace(re, '<mark>$1</mark>');
  },

  _showDropdown(html) {
    let dropdown = document.getElementById('search-dropdown');
    if (!dropdown) {
      dropdown = document.createElement('div');
      dropdown.id = 'search-dropdown';
      const searchBox = document.querySelector('.search-box');
      if (searchBox) {
        searchBox.style.position = 'relative';
        searchBox.appendChild(dropdown);
      } else {
        document.body.appendChild(dropdown);
      }
    }
    dropdown.innerHTML = html;
    dropdown.classList.add('visible');
    this._dropdownVisible = true;
  },

  _hideDropdown() {
    const dropdown = document.getElementById('search-dropdown');
    if (dropdown) {
      dropdown.classList.remove('visible');
    }
    this._dropdownVisible = false;
    this._lastQuery = '';
  },

  _bindResultClicks() {
    const dropdown = document.getElementById('search-dropdown');
    if (!dropdown) return;
    dropdown.querySelectorAll('.search-result').forEach(el => {
      el.addEventListener('click', async () => {
        const type = el.dataset.type;
        const id = parseInt(el.dataset.id, 10);
        this._hideDropdown();
        if (type === 'job') {
          App.currentJobId = id;
          await App.switchView('job');
        } else if (type === 'artifact') {
          App.currentArtifactId = id;
          await App.switchView('artifact');
        }
      });
    });
  },

  /** Perform server-side FTS for a view. Returns filtered job list. */
  async getServerSideJobs(query, status, category) {
    if (!query || query.length < 2) return null; // fallback to client
    try {
      return await DB.searchJobs(query, status, category);
    } catch {
      return null; // fallback to client
    }
  },
};
