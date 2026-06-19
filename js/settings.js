/* === Settings / Profile Page === */
const Settings = {
  render() {
    const container = document.getElementById('settings-container');
    const profile = DB.getProfile();
    const settings = DB.getSettings();

    container.innerHTML = `
      <div class="settings-section">
        <h3>${icon('user', 20)} User Profile</h3>
        <p class="text-muted text-sm mb-8" style="margin-bottom:16px;line-height:1.6">Your profile powers the Skills generators — emails, cover letters, interview prep, and career summaries are all personalized using your name, skills, experience, and preferences. Keep it updated for the best results.</p>
        <div class="settings-grid">
          <div class="form-group"><label>Full Name</label><input id="profile-name" value="${UI.escapeHtml(profile.name)}" /></div>
          <div class="form-group"><label>Email</label><input id="profile-email" type="email" value="${UI.escapeHtml(profile.email)}" /></div>
          <div class="form-group"><label>Phone</label><input id="profile-phone" value="${UI.escapeHtml(profile.phone)}" /></div>
          <div class="form-group"><label>Professional Title</label><input id="profile-title" value="${UI.escapeHtml(profile.title)}" /></div>
          <div class="form-group"><label>Skills (comma separated)</label><input id="profile-skills" value="${UI.escapeHtml((profile.skills || []).join(', '))}" /></div>
          <div class="form-group"><label>Industry</label><input id="profile-industry" value="${UI.escapeHtml(profile.industry)}" /></div>
          <div class="form-group full-width"><label>Education</label><textarea id="profile-education" rows="3">${UI.escapeHtml((profile.education || []).join('\n'))}</textarea></div>
          <div class="form-group full-width"><label>Experience</label><textarea id="profile-experience" rows="3">${UI.escapeHtml((profile.experience || []).join('\n'))}</textarea></div>
          <div class="form-group"><label>Preferred Greeting Style</label>
            <select id="profile-greeting">
              <option value="formal" ${profile.greetingStyle === 'formal' ? 'selected' : ''}>Formal</option>
              <option value="casual" ${profile.greetingStyle === 'casual' ? 'selected' : ''}>Casual</option>
              <option value="creative" ${profile.greetingStyle === 'creative' ? 'selected' : ''}>Creative</option>
            </select>
          </div>
          <div class="form-group"><label>Preferred Sign-Off</label><input id="profile-signoff" value="${UI.escapeHtml(profile.signOff || 'Best regards')}" /></div>
          <div class="full-width">
            <button class="btn btn-primary" id="profile-save">Save Profile</button>
          </div>
        </div>
      </div>
      <div class="settings-section">
        <h3>${icon('sliders', 20)} App Settings</h3>
        <div class="settings-grid">
          <div class="form-group"><label>Default View</label>
            <select id="settings-default-view">
              <option value="dashboard" ${settings.defaultView === 'dashboard' ? 'selected' : ''}>Dashboard</option>
              <option value="kanban" ${settings.defaultView === 'kanban' ? 'selected' : ''}>Kanban</option>
              <option value="table" ${settings.defaultView === 'table' ? 'selected' : ''}>Table</option>
              <option value="timeline" ${settings.defaultView === 'timeline' ? 'selected' : ''}>Timeline</option>
            </select>
          </div>
          <div class="form-group" style="display:flex;align-items:flex-end;padding-bottom:8px">
            <label style="margin-bottom:0;font-size:13px">
              <input type="checkbox" id="settings-reminders" ${settings.remindersEnabled ? 'checked' : ''} /> Enable Browser Notifications
            </label>
          </div>
          <div class="full-width">
            <button class="btn btn-primary" id="settings-save">Save Settings</button>
          </div>
        </div>
      </div>
      <div class="settings-section">
        <h3>${icon('save', 20)} Data Management</h3>
        <p class="text-muted text-sm mb-8">Export your data as JSON for backup. Import to restore from a backup file.</p>
        <div class="flex gap-8">
          <button class="btn btn-primary" id="settings-export">${icon('share', 16)} Export Data</button>
          <button class="btn btn-secondary" id="settings-import">${icon('download', 16)} Import Data</button>
        </div>
      </div>
      <div class="settings-section">
        <h3>${icon('zap', 20)} AI Integration</h3>
        <p class="text-muted text-sm" style="margin-bottom:12px;line-height:1.6">Enable AI-powered generation for smarter, more contextual skills output. When AI is enabled, the skill generators will use Google Gemini to create content. If the AI call fails (e.g. no internet or invalid key), it falls back to the built-in template generator automatically.</p>
        <div class="settings-grid">
          <div class="form-group full-width">
            <label style="font-size:13px">
              <input type="checkbox" id="ai-enabled" ${settings.aiEnabled ? 'checked' : ''} />
              Enable AI generation
            </label>
          </div>
          <div class="form-group full-width"><label>Gemini API Key</label><input type="password" id="gemini-key" value="${UI.escapeHtml(settings.geminiKey)}" placeholder="Paste your API key here" /></div>
          <div class="form-group full-width"><label>Model</label>
            <select id="gemini-model">
              <option value="gemini-1.5-flash" ${settings.geminiModel === 'gemini-1.5-flash' ? 'selected' : ''}>Gemini 1.5 Flash (Fast, free)</option>
              <option value="gemini-1.5-pro" ${settings.geminiModel === 'gemini-1.5-pro' ? 'selected' : ''}>Gemini 1.5 Pro (More capable)</option>
              <option value="gemini-2.0-flash" ${settings.geminiModel === 'gemini-2.0-flash' ? 'selected' : ''}>Gemini 2.0 Flash (Latest)</option>
            </select>
          </div>
          <div class="full-width" style="padding:12px;background:var(--accent-light);border-radius:var(--radius-md);font-size:13px;line-height:1.6">
            <strong>How to get a free Gemini API key:</strong><br />
            1. Go to <a href="https://aistudio.google.com/apikey" target="_blank" style="color:var(--accent);text-decoration:underline">aistudio.google.com/apikey</a><br />
            2. Sign in with your Google account<br />
            3. Click "Create API key" — no credit card needed<br />
            4. Copy the key and paste it above<br />
            5. Toggle "Enable AI generation" and save<br /><br />
            <em>Gemini 1.5 Flash has a generous free tier (60 requests/minute, 1500 requests/day). Your key stays on your device and is never sent anywhere except to Google's API.</em>
          </div>
          <div class="full-width">
            <button class="btn btn-primary" id="ai-save">Save AI Settings</button>
          </div>
        </div>
      </div>
    `;

    document.getElementById('profile-save').addEventListener('click', () => {
      const p = {
        name: document.getElementById('profile-name').value,
        email: document.getElementById('profile-email').value,
        phone: document.getElementById('profile-phone').value,
        title: document.getElementById('profile-title').value,
        skills: document.getElementById('profile-skills').value.split(',').map(s => s.trim()).filter(Boolean),
        industry: document.getElementById('profile-industry').value,
        education: document.getElementById('profile-education').value.split('\n').filter(Boolean),
        experience: document.getElementById('profile-experience').value.split('\n').filter(Boolean),
        greetingStyle: document.getElementById('profile-greeting').value,
        signOff: document.getElementById('profile-signoff').value,
      };
      DB.saveProfile(p);
      UI.showToast('Profile saved!', 'success');
    });

    document.getElementById('settings-save').addEventListener('click', () => {
      const s = {
        theme: document.documentElement.dataset.theme,
        defaultView: document.getElementById('settings-default-view').value,
        remindersEnabled: document.getElementById('settings-reminders').checked,
      };
      DB.saveSettings(s);
      UI.showToast('Settings saved!', 'success');
    });

    document.getElementById('ai-save').addEventListener('click', () => {
      const s = DB.getSettings();
      s.geminiKey = document.getElementById('gemini-key').value;
      s.geminiModel = document.getElementById('gemini-model').value;
      s.aiEnabled = document.getElementById('ai-enabled').checked;
      DB.saveSettings(s);
      UI.showToast('AI settings saved!', 'success');
    });

    document.getElementById('settings-export').addEventListener('click', Export.exportData);
    document.getElementById('settings-import').addEventListener('click', () => document.getElementById('import-file').click());

    document.getElementById('view-title').textContent = 'Settings';
  },
};
