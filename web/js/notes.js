/* === Notes Module === */
const Notes = {
  renderPreview(text, containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;
    if (!text || !text.trim()) {
      container.style.display = 'none';
      return;
    }
    container.innerHTML = UI.renderMarkdown(text);
    container.style.display = 'block';
  },

  initPreview() {
    const notesEl = document.getElementById('job-notes');
    if (notesEl) {
      notesEl.addEventListener('input', () => {
        this.renderPreview(notesEl.value, 'notes-preview');
      });
    }
  },
};
