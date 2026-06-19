/* === Generated Content View === */
const GeneratedContentView = {
  async render() {
    const container = document.getElementById('generated-container');
    const items = await DB.getGeneratedContent();

    document.getElementById('view-title').textContent = 'Generated Content';

    if (items.length === 0) {
      UI.showEmptyState(container, icon('folder', 48), 'No generated content yet', 'Use the Skills section to generate emails, cover letters, interview prep, and more.');
      return;
    }

    container.innerHTML = items.map(item => `
      <div class="generated-item" data-id="${item.id}">
        <div class="gen-header">
          <div>
            <div class="gen-title">${UI.escapeHtml(item.skillName)}</div>
            <div class="gen-meta">
              ${UI.escapeHtml(item.jobCompany)} - ${UI.escapeHtml(item.jobPosition)}
              · ${UI.formatDateTime(item.createdAt)}
            </div>
          </div>
          <div class="gen-actions">
            <button class="btn btn-sm btn-secondary copy-gen-btn" data-content="${UI.escapeHtml(item.content)}">${icon('copy', 14)} Copy</button>
            <button class="btn btn-sm btn-danger delete-gen-btn" data-id="${item.id}">${icon('trash', 14)}</button>
          </div>
        </div>
        <div class="gen-content">${UI.escapeHtml(item.content)}</div>
        <div style="font-size:11px;color:var(--text-muted)">${icon('zap', 12)} ${stripEmoji(item.title) || ''}</div>
      </div>
    `).join('');

    container.querySelectorAll('.copy-gen-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        navigator.clipboard.writeText(btn.dataset.content).then(() => {
          UI.showToast('Copied to clipboard!', 'success');
        });
      });
    });

    container.querySelectorAll('.delete-gen-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        const confirmed = await UI.confirmDialog('Delete this generated content?');
        if (confirmed) {
          DB.deleteGeneratedContent(btn.dataset.id);
          this.render();
          UI.showToast('Deleted', 'success');
        }
      });
    });
  },
};
