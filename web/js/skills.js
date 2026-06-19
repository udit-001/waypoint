/* === Skills (read-only catalog) === */
const Skills = {
  init() {
    // nothing to preload — view is static
  },

  getSkillsConfig() {
    return [
      {
        id: 'email-generator',
        name: 'Email Template Generator',
        desc: 'Creates personalized application emails, follow-up emails, and thank you emails based on job details and user profile.',
        icon: 'mail',
      },
      {
        id: 'cover-letter',
        name: 'Cover Letter Generator',
        desc: 'Creates tailored cover letters in multiple versions - formal, casual, creative.',
        icon: 'file-text',
      },
      {
        id: 'resume-optimizer',
        name: 'Resume Keyword Optimizer',
        desc: 'Analyzes job descriptions, suggests keywords to add, provides match score and gap analysis.',
        icon: 'search',
      },
      {
        id: 'interview-prep',
        name: 'Interview Prep Assistant',
        desc: 'Generates role-specific interview questions, sample answers, and research checklist.',
        icon: 'target',
      },
      {
        id: 'career-summary',
        name: 'Career Summary Generator',
        desc: 'Creates professional resume summaries in multiple versions based on experience and target role.',
        icon: 'star',
      },
    ];
  },

  async renderList() {
    const container = document.getElementById('skills-container');
    const configs = this.getSkillsConfig();

    container.innerHTML = `
      <p class="text-muted text-sm" style="margin-bottom:16px">
        Built-in skills available in the tracker. Generation is managed via the CLI.
      </p>
      ${configs.map(s => `
        <div class="skill-card">
          <div class="skill-card-header">
            <h3>${icon(s.icon, 18)} ${s.name}</h3>
          </div>
          <p>${s.desc}</p>
        </div>
      `).join('')}
    `;

    document.getElementById('view-title').textContent = 'Skills';
  },
};
