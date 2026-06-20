/* === Categories Page === */
const CategoriesView = {

  async render() {
    const container = document.getElementById('categories-container');
    const cats = await DB.getCategories();
    const jobs = await DB.getJobs();

    // Count jobs per category name
    const jobCounts = {};
    jobs.forEach(j => {
      const cat = j.category || 'General';
      jobCounts[cat] = (jobCounts[cat] || 0) + 1;
    });

    container.innerHTML = `
      <p class="text-muted text-sm" style="margin-bottom:16px">
        Organize your applications into categories. Manage them via the CLI.
      </p>

      <div class="cat-cli-box">
        <h4>${icon('terminal', 16)} CLI Commands</h4>
        <pre class="cat-cli-pre"><code>waypoint categories list              # List all categories
waypoint categories add "Remote"       # Add a category
waypoint categories rename 2 "Tech"    # Rename by ID
waypoint categories delete 3           # Delete by ID (jobs → General)</code></pre>
      </div>

      <h3 style="margin:24px 0 12px">${icon('box', 18)} All Categories</h3>

      ${cats.length === 0 ? '<p class="text-muted text-sm">No categories yet.</p>' : `
      <table class="cat-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Jobs</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          ${cats.map(c => `
            <tr>
              <td><a href="/table" class="cat-job-link" data-cat="${UI.escapeHtml(c.name)}">${UI.escapeHtml(c.name)}</a>${c.id === 1 ? ' <span class="cat-default-badge">default</span>' : ''}</td>
              <td><a href="/table" class="cat-job-link" data-cat="${UI.escapeHtml(c.name)}">${jobCounts[c.name] || 0}</a></td>
              <td class="cat-actions">
                <code class="cat-cmd" title="Copy" data-cmd="waypoint categories rename ${c.id} &quot;New Name&quot;">rename</code>
                ${c.id !== 1 ? `<code class="cat-cmd cat-cmd-danger" title="Copy" data-cmd="waypoint categories delete ${c.id}">delete</code>` : ''}
              </td>
            </tr>
          `).join('')}
        </tbody>
      </table>
      `}
    `;

    document.getElementById('view-title').textContent = 'Categories';

    // Job count links → table view with category filter
    container.querySelectorAll('.cat-job-link').forEach(a => {
      a.addEventListener('click', async e => {
        e.preventDefault();
        App.tableCategoryFilter = a.dataset.cat;
        await App.switchView('table');
      });
    });

    // Click-to-copy on action commands
    container.querySelectorAll('.cat-cmd').forEach(el => {
      el.addEventListener('click', () => {
        const cmd = el.dataset.cmd;
        navigator.clipboard.writeText(cmd).then(() => {
          const orig = el.textContent;
          el.textContent = '✓ copied';
          setTimeout(() => { el.textContent = orig; }, 1200);
        });
      });
    });
  },
};
