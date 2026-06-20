/* === AI Integration (formerly Skills) === */
const Skills = {
  init() {},

  getSkillsConfig() {
    return [
      {
        id: 'email-generator',
        name: 'Email Template Generator',
        desc: 'Personalized application, follow-up, thank-you, networking, and referral emails.',
        icon: 'mail',
        tags: ['7 email types', '4 tones', 'Auto-signature'],
      },
      {
        id: 'cover-letter',
        name: 'Cover Letter Generator',
        desc: 'Tailored cover letters in formal, casual, creative, and executive styles.',
        icon: 'file-text',
        tags: ['4 tones', '3 lengths', 'Skill emphasis'],
      },
      {
        id: 'resume-optimizer',
        name: 'Resume Keyword Optimizer',
        desc: 'Keyword match scoring, gap analysis, and action-verb suggestions against a job posting.',
        icon: 'search',
        tags: ['Match score', 'Gap analysis', 'Action verbs'],
      },
      {
        id: 'interview-prep',
        name: 'Interview Prep Assistant',
        desc: 'Role-specific questions, sample answers (STAR), and company research checklists.',
        icon: 'target',
        tags: ['6 interview types', '3 difficulty levels', 'Research checklist'],
      },
      {
        id: 'career-summary',
        name: 'Career Summary Generator',
        desc: 'Resume summaries in standard, impact, technical, executive, or entry-level styles.',
        icon: 'star',
        tags: ['5 styles', '3 lengths', 'Target role'],
      },
    ];
  },

  getAgents() {
    return [
      { id: 'pi.dev',     name: 'Pi',          dir: '.pi/skills/waypoint' },
      { id: 'claude-code', name: 'Claude Code', dir: '.claude/skills/waypoint' },
      { id: 'codex',       name: 'Codex',       dir: '.codex/skills/waypoint' },
      { id: 'opencode',    name: 'OpenCode',    dir: '.opencode/skills/waypoint' },
    ];
  },

  async renderList() {
    const container = document.getElementById('skills-container');
    const configs = this.getSkillsConfig();
    const agents = this.getAgents();

    container.innerHTML = `
      <p class="text-muted text-sm" style="margin-bottom:16px">
        Connect your AI coding agent to Waypoint — it learns the CLI commands and can generate job-search content on demand.
      </p>

      <div class="ai-install-section">
        <h3>${icon('brain', 18)} Install the Waypoint skill</h3>
        <p class="text-muted text-sm">Run this in your project directory. The skill teaches your agent how to use <code>waypoint</code> and provides generation references.</p>
        <div class="ai-install-cmd">
          <code id="install-cmd">waypoint skills install --agent pi.dev</code>
          <button class="btn btn-sm btn-secondary" id="copy-install-cmd" title="Copy command">${icon('copy', 14)} Copy</button>
        </div>
        <div class="ai-agent-pills">
          ${agents.map(a => `
            <button class="ai-agent-pill${a.id === 'pi.dev' ? ' active' : ''}" data-agent="${a.id}">${a.name}</button>
          `).join('')}
          <span class="text-muted text-xs" style="margin-left:4px">→ installs to <code id="install-dir">.pi/skills/waypoint/</code></span>
        </div>
      </div>

      <h3 style="margin:24px 0 12px">${icon('zap', 18)} Available skills</h3>

      ${configs.map(s => `
        <div class="skill-card">
          <div class="skill-card-header">
            <h3>${icon(s.icon, 18)} ${s.name}</h3>
          </div>
          <p>${s.desc}</p>
          <div class="skill-meta">
            ${s.tags.map(t => `<span class="skill-tag">${t}</span>`).join('')}
          </div>
        </div>
      `).join('')}
    `;

    document.getElementById('view-title').textContent = 'AI Integration';

    // Agent pill switching
    container.querySelectorAll('.ai-agent-pill').forEach(pill => {
      pill.addEventListener('click', () => {
        container.querySelectorAll('.ai-agent-pill').forEach(p => p.classList.remove('active'));
        pill.classList.add('active');
        const agentId = pill.dataset.agent;
        const agent = agents.find(a => a.id === agentId);
        document.getElementById('install-cmd').textContent = `waypoint skills install --agent ${agentId}`;
        document.getElementById('install-dir').textContent = agent.dir + '/';
      });
    });

    // Copy install command
    document.getElementById('copy-install-cmd').addEventListener('click', () => {
      const cmd = document.getElementById('install-cmd').textContent;
      navigator.clipboard.writeText(cmd).then(() => {
        const btn = document.getElementById('copy-install-cmd');
        btn.innerHTML = `${icon('check', 14)} Copied`;
        setTimeout(() => { btn.innerHTML = `${icon('copy', 14)} Copy`; }, 1500);
      });
    });
  },
};
