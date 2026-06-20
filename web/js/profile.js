/* === Profile Page === */
const ProfileView = {

  async render() {
    const container = document.getElementById('profile-container');
    const profile = await DB.getProfile();

    if (!profile) {
      container.innerHTML = '<p class="text-muted text-sm">Profile not loaded (server may be down).</p>';
      document.getElementById('view-title').textContent = 'Profile';
      return;
    }

    const skills = Array.isArray(profile.skills) ? profile.skills : [];
    const education = Array.isArray(profile.education) ? profile.education : [];
    const experience = Array.isArray(profile.experience) ? profile.experience : [];

    container.innerHTML = `
      <p class="text-muted text-sm" style="margin-bottom:16px">
        Your profile personalizes AI-generated content. Manage it via the CLI.
      </p>

      <div class="cat-cli-box">
        <h4>${icon('terminal', 16)} CLI Commands</h4>
        <pre class="cat-cli-pre"><code>waypoint profile show                                    # View profile
waypoint profile set --name "Jane Doe" --title "SWE"     # Update fields
waypoint profile set --skills '["Go","React","AWS"]'     # JSON arrays
waypoint profile set --greeting-style casual             # Email style</code></pre>
      </div>

      <div class="profile-section">
        <h3>${icon('user', 18)} Personal Info</h3>
        <div class="settings-grid">
          <div class="form-group"><label>Full Name</label><div class="readonly-field">${UI.escapeHtml(profile.name || '-')}</div></div>
          <div class="form-group"><label>Professional Title</label><div class="readonly-field">${UI.escapeHtml(profile.title || '-')}</div></div>
          <div class="form-group"><label>Email</label><div class="readonly-field">${UI.escapeHtml(profile.email || '-')}</div></div>
          <div class="form-group"><label>Phone</label><div class="readonly-field">${UI.escapeHtml(profile.phone || '-')}</div></div>
          <div class="form-group"><label>Industry</label><div class="readonly-field">${UI.escapeHtml(profile.industry || '-')}</div></div>
          <div class="form-group"><label>Greeting Style</label><div class="readonly-field">${UI.escapeHtml(profile.greetingStyle || 'formal')}</div></div>
        </div>
      </div>

      <div class="profile-section">
        <h3>${icon('zap', 18)} Skills</h3>
        ${skills.length > 0 ? `<div class="profile-tags">${skills.map(s => `<span class="skill-tag">${UI.escapeHtml(s)}</span>`).join('')}</div>` : '<p class="text-muted text-sm">No skills set yet.</p>'}
      </div>

      <div class="profile-section">
        <h3>${icon('grad', 18)} Education</h3>
        ${education.length > 0 ? `<ul class="profile-list">${education.map(e => `<li>${UI.escapeHtml(e)}</li>`).join('')}</ul>` : '<p class="text-muted text-sm">No education set yet.</p>'}
      </div>

      <div class="profile-section">
        <h3>${icon('briefcase', 18)} Experience</h3>
        ${experience.length > 0 ? `<ul class="profile-list">${experience.map(e => `<li>${UI.escapeHtml(e)}</li>`).join('')}</ul>` : '<p class="text-muted text-sm">No experience set yet.</p>'}
      </div>

      <div class="profile-section">
        <h3>${icon('mail', 18)} Email Preferences</h3>
        <div class="settings-grid">
          <div class="form-group"><label>Sign-Off</label><div class="readonly-field">${UI.escapeHtml(profile.signOff || 'Best regards')}</div></div>
        </div>
      </div>
    `;

    document.getElementById('view-title').textContent = 'Profile';
  },
};
