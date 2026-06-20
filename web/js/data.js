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

  async searchJobs(query, status, category) {
    const params = new URLSearchParams();
    params.set('search', query);
    if (status) params.set('status', status);
    if (category) params.set('category', category);
    const data = await this._api('/jobs?' + params.toString());
    return Array.isArray(data) ? data : [];
  },

  async searchAll(query) {
    const data = await this._api('/search?q=' + encodeURIComponent(query));
    return Array.isArray(data) ? data : [];
  },

  async getJob(id) {
    return this._api(`/jobs/${id}`);
  },

  addJob(job) {
    UI.showToast('Use the CLI to add jobs: waypoint jobs add "Company" "Position"', 'info');
    return Promise.resolve(job);
  },

  updateJob(id, updates) {
    UI.showToast('Use the CLI to update jobs: waypoint jobs update ' + id + ' --flag value', 'info');
    return Promise.resolve(null);
  },

  deleteJob(id) {
    UI.showToast('Use the CLI to delete jobs: waypoint jobs delete ' + id, 'info');
    return Promise.resolve(true);
  },

  // === Categories ===

  async getCategories() {
    const data = await this._api('/categories');
    if (!Array.isArray(data)) return [{ id: 1, name: 'General' }];
    return data;
  },

  addCategory(name) {
    UI.showToast('Use the CLI: waypoint categories add "', 'info');
    return Promise.resolve(false);
  },

  deleteCategory(name) {
    UI.showToast('Use the CLI: waypoint categories delete "', 'info');
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

  // === Artifacts ===

  async getArtifacts(skill, job) {
    let path = '/artifacts';
    const params = new URLSearchParams();
    if (skill) params.set('skill', skill);
    if (job) params.set('job', job);
    const qs = params.toString();
    if (qs) path += '?' + qs;
    const data = await this._api(path);
    return Array.isArray(data) ? data : [];
  },

  async searchArtifacts(query) {
    const data = await this._api('/artifacts?search=' + encodeURIComponent(query));
    return Array.isArray(data) ? data : [];
  },

  async getArtifact(id) {
    return this._api(`/artifacts/${id}`);
  },

  // === Generated Content (legacy alias for the view) ===

  async getGeneratedContent() {
    return this.getArtifacts();
  },

  // === Skill Feedback ===

  getSkillFeedback() {
    return Promise.resolve({});
  },

  addSkillFeedback(skillName, contentHash, rating) {
    return Promise.resolve();
  },
};
