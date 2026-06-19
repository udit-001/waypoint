/* === Data Layer — Read-only, backed by the CLI's API === */

const DB = {
  keys: {},

  _api(path) {
    return fetch('/api' + path).then(r => {
      if (!r.ok) throw new Error(r.statusText);
      return r.json();
    }).catch(() => {
      // Server offline — show empty state
      return null;
    });
  },

  // === Jobs ===

  async getJobs() {
    const data = await this._api('/jobs');
    return Array.isArray(data) ? data : [];
  },

  async getJob(id) {
    return this._api(`/jobs/${id}`);
  },

  addJob(job) {
    UI.showToast('Use the CLI to add jobs: job-tracker add "Company" "Position"', 'info');
    return Promise.resolve(job);
  },

  updateJob(id, updates) {
    UI.showToast('Use the CLI to update jobs: job-tracker update ' + id + ' --flag value', 'info');
    return Promise.resolve(null);
  },

  deleteJob(id) {
    UI.showToast('Use the CLI to delete jobs: job-tracker delete ' + id, 'info');
    return Promise.resolve(true);
  },

  // === Categories ===

  async getCategories() {
    const data = await this._api('/categories');
    if (!Array.isArray(data)) return ['General'];
    return data.map(c => c.name);
  },

  addCategory(name) {
    UI.showToast('Use the CLI: job-tracker add-category "', 'info');
    return Promise.resolve(false);
  },

  deleteCategory(name) {
    UI.showToast('Use the CLI: job-tracker delete-category "', 'info');
    return Promise.resolve(false);
  },

  // === History ===

  async getHistory() {
    const data = await this._api('/history');
    return Array.isArray(data) ? data : [];
  },

  async getJobHistory(jobId) {
    const data = await this._api(`/jobs/${jobId}/history`);
    return Array.isArray(data) ? data : [];
  },

  addHistory(jobId, action, from, to) {
    return Promise.resolve();
  },

  // === Profile ===

  async getProfile() {
    const p = await this._api('/profile');
    if (!p) return null;
    // Parse JSON array fields stored as strings in SQLite
    if (typeof p.skills === 'string') try { p.skills = JSON.parse(p.skills); } catch { p.skills = []; }
    if (typeof p.experience === 'string') try { p.experience = JSON.parse(p.experience); } catch { p.experience = []; }
    if (typeof p.education === 'string') try { p.education = JSON.parse(p.education); } catch { p.education = []; }
    return p;
  },

  saveProfile(p) {
    UI.showToast('Use the CLI to update your profile', 'info');
    return Promise.resolve();
  },

  // === Settings ===

  async getSettings() {
    const s = await this._api('/settings');
    if (!s) {
      return {
        theme: 'light',
        remindersEnabled: true,
        defaultView: 'dashboard',
        itemsPerPage: 25,
      };
    }
    return {
      theme: s.theme || 'light',
      remindersEnabled: Boolean(s.remindersEnabled),
      defaultView: s.defaultView || 'dashboard',
      itemsPerPage: s.itemsPerPage || 25,
      geminiKey: '',
      geminiModel: 'gemini-1.5-flash',
      aiEnabled: false,
    };
  },

  saveSettings(s) {
    UI.showToast('Use the CLI to update settings', 'info');
    return Promise.resolve();
  },

  // === Generated Content ===

  getGeneratedContent() {
    return Promise.resolve([]);
  },

  addGeneratedContent(item) {
    UI.showToast('Content generation is managed via the CLI', 'info');
    return Promise.resolve(item);
  },

  deleteGeneratedContent(id) {
    return Promise.resolve();
  },

  // === Skill Feedback ===

  getSkillFeedback() {
    return Promise.resolve({});
  },

  addSkillFeedback(skillName, contentHash, rating) {
    return Promise.resolve();
  },
};
