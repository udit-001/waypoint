/* === Export / Import === */
const Export = {
  exportData() {
    const data = {
      version: '1.0',
      exportedAt: new Date().toISOString(),
      jobs: DB.getJobs(),
      categories: DB.getCategories(),
      history: DB.getHistory(),
      profile: DB.getProfile(),
      settings: DB.getSettings(),
      generatedContent: DB.getGeneratedContent(),
      skillFeedback: DB.getSkillFeedback(),
    };
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `job-tracker-backup-${new Date().toISOString().split('T')[0]}.json`;
    a.click();
    URL.revokeObjectURL(url);
    UI.showToast('Data exported successfully!', 'success');
  },

  importData(e) {
    const file = e.target.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = async (evt) => {
      try {
        const data = JSON.parse(evt.target.result);
        if (!data.jobs || !data.categories) {
          UI.showToast('Invalid backup file format', 'error');
          return;
        }
        const confirmed = await UI.confirmDialog('This will replace all current data. Are you sure?');
        if (!confirmed) return;
        if (data.jobs) DB.saveJobs(data.jobs);
        if (data.categories) DB.saveCategories(data.categories);
        if (data.history) DB.saveHistory(data.history);
        if (data.profile) DB.saveProfile(data.profile);
        if (data.settings) DB.saveSettings(data.settings);
        if (data.generatedContent) DB.saveGeneratedContent(data.generatedContent);
        if (data.skillFeedback) DB.saveSkillFeedback(data.skillFeedback);
        UI.showToast('Data imported successfully!', 'success');
        App.init();
      } catch (err) {
        UI.showToast('Error importing data: ' + err.message, 'error');
      }
    };
    reader.readAsText(file);
    e.target.value = '';
  },
};
