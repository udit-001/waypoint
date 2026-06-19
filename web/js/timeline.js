/* === Timeline View (read-only) === */
const Timeline = {
  async render() {
    const container = document.getElementById('timeline-container');
    const allHistory = await DB.getHistory();
    let jobs = await DB.getJobs();

    if (App.currentCategory && App.currentCategory !== 'all') {
      const catJobIds = new Set(jobs.filter(j => j.category === App.currentCategory).map(j => j.id));
      if (App.searchQuery) {
        const q = App.searchQuery.toLowerCase();
        jobs = jobs.filter(j => catJobIds.has(j.id) && (
          j.company.toLowerCase().includes(q) ||
          j.position.toLowerCase().includes(q) ||
          (j.notes && j.notes.toLowerCase().includes(q))
        ));
      }
      const filteredIds = new Set(jobs.map(j => j.id));
      const relevantHistory = allHistory.filter(h => filteredIds.has(h.jobId));
      this.renderTimeline(container, relevantHistory, jobs);
    } else if (App.searchQuery) {
      const q = App.searchQuery.toLowerCase();
      jobs = jobs.filter(j =>
        j.company.toLowerCase().includes(q) ||
        j.position.toLowerCase().includes(q) ||
        (j.notes && j.notes.toLowerCase().includes(q))
      );
      const filteredIds = new Set(jobs.map(j => j.id));
      const relevantHistory = allHistory.filter(h => filteredIds.has(h.jobId));
      this.renderTimeline(container, relevantHistory, jobs);
    } else {
      this.renderTimeline(container, allHistory, jobs);
    }

    document.getElementById('view-title').textContent = 'Timeline';
  },

  renderTimeline(container, history, jobs) {
    const jobMap = {};
    jobs.forEach(j => jobMap[j.id] = j);

    if (history.length === 0) {
      UI.showEmptyState(container, icon('timeline', 48), 'No activity yet', 'Job changes and updates will appear here chronologically.');
      return;
    }

    container.innerHTML = '';
    history.forEach(h => {
      const job = jobMap[h.jobId];
      if (!job) return;
      const item = document.createElement('div');
      item.className = 'timeline-item';
      item.style.cursor = 'pointer';
      item.innerHTML = `
        <div class="timeline-date">${UI.formatDateTime(h.timestamp)}</div>
        <div class="timeline-content">
          <div>
            <span class="timeline-company">${UI.escapeHtml(job.company)}</span>
            <span class="timeline-position">- ${UI.escapeHtml(job.position)}</span>
          </div>
          <div style="margin-top:2px;font-size:13px">
            ${h.action === 'Created' ? `${icon('plus', 14)} Job created as ${UI.statusBadge(job.status)}` :
              h.action === 'Status' ? `${icon('pin', 14)} Status changed: <span class="history-from">${UI.escapeHtml(h.from)}</span> → <span class="history-to">${UI.escapeHtml(h.to)}</span>` :
              h.action === 'Deleted' ? `${icon('trash', 14)} Job deleted` :
              `${icon('edit', 14)} Job updated`}
          </div>
        </div>
      `;
      item.addEventListener('click', () => App.showJobDetail(job.id));
      container.appendChild(item);
    });
  },
};
