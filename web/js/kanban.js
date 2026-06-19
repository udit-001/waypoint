/* === Kanban Board View (read-only, no drag) === */
const Kanban = {
  statuses: ['Not Applied', 'Applied', 'Offer', 'Rejected', 'Withdrawn'],

  async render() {
    const board = document.getElementById('kanban-board');
    board.innerHTML = '';
    const jobs = await this.getFilteredJobs();
    this.statuses.forEach(status => {
      const col = document.createElement('div');
      col.className = 'kanban-column';
      col.dataset.status = status;
      const colJobs = jobs.filter(j => j.status === status);
      col.innerHTML = `
        <div class="kanban-column-header">
          <span>${status}</span>
          <span class="kanban-count">${colJobs.length}</span>
        </div>
        <div class="kanban-cards" data-status="${status}"></div>
      `;
      const cardsContainer = col.querySelector('.kanban-cards');
      colJobs.forEach(job => {
        cardsContainer.appendChild(this.createCard(job));
      });
      board.appendChild(col);
    });

    document.getElementById('view-title').textContent = 'Kanban Board';
  },

  createCard(job) {
    const card = document.createElement('div');
    card.className = 'kanban-card';
    card.draggable = false; // read-only
    card.dataset.jobId = job.id;
    card.innerHTML = `
      <div class="card-company">${UI.escapeHtml(job.company)}</div>
      <div class="card-position">${UI.escapeHtml(job.position)}</div>
      <div class="card-meta">
        <span>${UI.formatDate(job.date)}</span>
        ${job.salary ? `<span>${UI.escapeHtml(job.salary)}</span>` : ''}
        ${job.location ? `<span>${UI.escapeHtml(job.location)}</span>` : ''}
        ${job.appliedDate ? `<span>Applied: ${UI.formatDate(job.appliedDate)}</span>` : ''}
      </div>
      <div>
        <span class="card-category">${UI.escapeHtml(job.category || 'General')}</span>
      </div>
    `;
    card.addEventListener('click', () => App.showJobDetail(job.id));
    return card;
  },

  async getFilteredJobs() {
    let jobs = await DB.getJobs();
    if (App.currentCategory && App.currentCategory !== 'all') {
      jobs = jobs.filter(j => j.category === App.currentCategory);
    }
    if (App.searchQuery) {
      const q = App.searchQuery.toLowerCase();
      jobs = jobs.filter(j =>
        j.company.toLowerCase().includes(q) ||
        j.position.toLowerCase().includes(q) ||
        (j.notes && j.notes.toLowerCase().includes(q)) ||
        (j.location && j.location.toLowerCase().includes(q))
      );
    }
    if (App.advancedFilters) {
      const f = App.advancedFilters;
      if (f.company) jobs = jobs.filter(j => j.company.toLowerCase().includes(f.company));
      if (f.position) jobs = jobs.filter(j => j.position.toLowerCase().includes(f.position));
      if (f.category) jobs = jobs.filter(j => j.category === f.category);
      if (f.status) jobs = jobs.filter(j => j.status === f.status);
      if (f.dateFrom) jobs = jobs.filter(j => j.date >= f.dateFrom);
      if (f.dateTo) jobs = jobs.filter(j => j.date <= f.dateTo);
      if (f.notes) jobs = jobs.filter(j => j.notes && j.notes.toLowerCase().includes(f.notes));
    }
    return jobs;
  },
};
