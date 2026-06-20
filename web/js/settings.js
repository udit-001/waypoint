/* === Settings / Profile Page (read-only) === */
const Settings = {
  async render() {
    const container = document.getElementById('settings-container');
    const profile = await DB.getProfile();
    const settings = await DB.getSettings();

    const profileHtml = profile ? `
      <div class="settings-section">
        <h3>${icon('user', 20)} User Profile</h3>
        <p class="text-muted text-sm mb-8" style="margin-bottom:16px;line-height:1.6">Your profile is managed via the CLI. Use <code>waypoint update-profile --help</code> to see available options.</p>
        <div class="settings-grid">
          <div class="form-group"><label>Full Name</label><div class="readonly-field">${UI.escapeHtml(profile.name || '-')}</div></div>
          <div class="form-group"><label>Email</label><div class="readonly-field">${UI.escapeHtml(profile.email || '-')}</div></div>
          <div class="form-group"><label>Phone</label><div class="readonly-field">${UI.escapeHtml(profile.phone || '-')}</div></div>
          <div class="form-group"><label>Professional Title</label><div class="readonly-field">${UI.escapeHtml(profile.title || '-')}</div></div>
          <div class="form-group"><label>Skills</label><div class="readonly-field">${UI.escapeHtml((profile.skills || []).join(', ') || '-')}</div></div>
          <div class="form-group"><label>Industry</label><div class="readonly-field">${UI.escapeHtml(profile.industry || '-')}</div></div>
          <div class="form-group full-width"><label>Education</label><div class="readonly-field pre">${UI.escapeHtml((profile.education || []).join('\n') || '-')}</div></div>
          <div class="form-group full-width"><label>Experience</label><div class="readonly-field pre">${UI.escapeHtml((profile.experience || []).join('\n') || '-')}</div></div>
          <div class="form-group"><label>Greeting Style</label><div class="readonly-field">${UI.escapeHtml(profile.greetingStyle || 'formal')}</div></div>
          <div class="form-group"><label>Sign-Off</label><div class="readonly-field">${UI.escapeHtml(profile.signOff || 'Best regards')}</div></div>
        </div>
      </div>
    ` : '<div class="settings-section"><p class="text-muted">Profile not loaded (server may be down).</p></div>';

    container.innerHTML = `
      ${profileHtml}
      <div class="settings-section">
        <h3>${icon('sliders', 20)} App Settings</h3>
        <p class="text-muted text-sm mb-8">Settings are managed via the CLI.</p>
        <div class="settings-grid">
          <div class="form-group"><label>Default View</label><div class="readonly-field">${UI.escapeHtml(settings.defaultView || 'dashboard')}</div></div>
          <div class="form-group"><label>Theme</label><div class="readonly-field">${UI.escapeHtml(settings.theme || 'light')}</div></div>
          <div class="form-group"><label>Notifications</label><div class="readonly-field">${settings.remindersEnabled ? 'Enabled' : 'Disabled'}</div></div>
          <div class="form-group"><label>Items Per Page</label><div class="readonly-field">${settings.itemsPerPage || 25}</div></div>
        </div>
      </div>
      <div class="settings-section" id="font-settings">
        <h3>${icon('type', 20)} Typography</h3>
        <p class="text-muted text-sm mb-8">Choose your preferred reading font.</p>
        <div class="font-options" style="display:flex;gap:12px">
          <button class="font-btn" data-font="sans" style="flex:1;padding:16px;border:2px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-secondary);cursor:pointer;text-align:center;transition:all 0.2s;font-family:'Inter',sans-serif">
            <div style="font-size:20px;font-weight:600;margin-bottom:4px">Aa</div>
            <div style="font-size:12px;opacity:0.7">Inter</div>
            <div style="font-size:11px;opacity:0.5;margin-top:2px">Sans-serif</div>
          </button>
          <button class="font-btn" data-font="serif" style="flex:1;padding:16px;border:2px solid var(--border-color);border-radius:var(--radius-md);background:var(--bg-secondary);cursor:pointer;text-align:center;transition:all 0.2s;font-family:'PT Serif',serif">
            <div style="font-size:20px;font-weight:600;margin-bottom:4px">Aa</div>
            <div style="font-size:12px;opacity:0.7">PT Serif</div>
            <div style="font-size:11px;opacity:0.5;margin-top:2px">Serif</div>
          </button>
        </div>
      </div>
      <div class="settings-section">
        <h3>${icon('terminal', 20)} CLI Quick Reference</h3>
        <pre style="background:var(--bg-secondary);padding:16px;border-radius:8px;font-size:13px;line-height:1.7;overflow-x:auto">
  waypoint add "Company" "Position" --status Applied --category Tech
  waypoint list --status Applied
  waypoint update 42 --status Offer --notes "Got the offer!"
  waypoint delete 42
  waypoint stats
  waypoint get 42 --history
        </pre>
      </div>
    `;

    document.getElementById('view-title').textContent = 'Settings';

    // Font toggle
    const currentFont = document.documentElement.dataset.font || 'serif';
    document.querySelectorAll('.font-btn').forEach(btn => {
      const font = btn.dataset.font;
      if (font === currentFont) {
        btn.style.borderColor = 'var(--accent)';
        btn.style.background = 'var(--bg-hover)';
      }
      btn.addEventListener('click', () => {
        document.documentElement.dataset.font = font;
        localStorage.setItem('waypoint_font', font);
        document.querySelectorAll('.font-btn').forEach(b => {
          b.style.borderColor = 'var(--border-color)';
          b.style.background = 'var(--bg-secondary)';
        });
        btn.style.borderColor = 'var(--accent)';
        btn.style.background = 'var(--bg-hover)';
      });
    });
  },
};
