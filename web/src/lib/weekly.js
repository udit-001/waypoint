// WP-97 — weekly aggregation for the velocity sparkline.
// Groups jobs by ISO week (Monday-start) of their appliedDate.
// Used by VelocityChart.svelte to render the 8-week trend.
//
// Pure functions: no DOM, no Date.now() — `now` is injectable for tests.

const MS_PER_DAY = 86400000;

/** ISO week-of-year for a date. Returns { year, week }.
 *  Week 1 is the week containing the first Thursday of the year. */
export function isoWeek(date) {
  const d = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
  const dayNum = d.getUTCDay() || 7; // Mon=1..Sun=7
  d.setUTCDate(d.getUTCDate() + 4 - dayNum); // nearest Thursday
  const yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
  const week = Math.ceil((((d - yearStart) / MS_PER_DAY) + 1) / 7);
  return { year: d.getUTCFullYear(), week };
}

/** Monday of the week containing `date` (local time, 00:00:00). */
export function weekStart(date) {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  const dayNum = d.getDay() || 7; // Mon=1..Sun=7
  d.setDate(d.getDate() - (dayNum - 1));
  return d;
}

/** Build the last `numWeeks` weeks (oldest -> newest), each with a count
 *  of jobs whose appliedDate falls in that week. Week starts Monday.
 *  Jobs without appliedDate or with an invalid date are ignored.
 *  Jobs older than the window are ignored. */
export function weeklyCounts(jobs, now = new Date(), numWeeks = 8) {
  if (!numWeeks || numWeeks < 1) return [];
  const thisWeek = weekStart(now);
  const weeks = [];
  for (let i = numWeeks - 1; i >= 0; i--) {
    const start = new Date(thisWeek);
    start.setDate(thisWeek.getDate() - i * 7);
    const { week } = isoWeek(start);
    weeks.push({
      label: 'W' + week,
      startMs: start.getTime(),
      count: 0,
      isCurrent: i === 0,
    });
  }
  const byStart = new Map(weeks.map(w => [w.startMs, w]));
  for (const job of jobs || []) {
    if (!job.appliedDate) continue;
    const d = new Date(job.appliedDate);
    if (isNaN(d)) continue;
    const w = byStart.get(weekStart(d).getTime());
    if (w) w.count++;
  }
  return weeks;
}

/** Summarize weeks for the chart header: total + average. */
export function weeklySummary(weeks) {
  const total = weeks.reduce((s, w) => s + w.count, 0);
  const avg = weeks.length ? total / weeks.length : 0;
  return { total, avg };
}
