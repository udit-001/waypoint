/* === Browser Notifications === */
const Notifications = {
  requestPermission() {
    if (!('Notification' in window)) return;
    if (Notification.permission === 'default') {
      Notification.requestPermission();
    }
  },

  scheduleReminder(job) {
    if (!('Notification' in window)) return;
    const settings = DB.getSettings();
    if (!settings.remindersEnabled) return;
    if (!job.reminderDate) return;
    const reminderTime = new Date(job.reminderDate).getTime();
    const now = Date.now();
    const delay = reminderTime - now;
    if (delay < 0) return;
    setTimeout(() => {
      if (Notification.permission === 'granted') {
        new Notification('Job Follow-Up Reminder', {
          body: `Follow up on ${job.company} - ${job.position}`,
          icon: ''  // No icon for notification - browsers don't support SVG here
        });
      }
    }, delay);
  },

  checkAllReminders() {
    const jobs = DB.getJobs();
    jobs.forEach(j => this.scheduleReminder(j));
  },
};
