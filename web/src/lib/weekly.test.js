import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { isoWeek, weekStart, weeklyCounts, weeklySummary } from './weekly.js';

const NOW = new Date('2026-07-19T12:00:00Z'); // Sunday

function daysAgoISO(d, now = NOW) {
  const dt = new Date(now);
  dt.setDate(dt.getDate() - d);
  return dt.toISOString();
}

describe('isoWeek', () => {
  it('returns ISO week number and year for a known date', () => {
    const w = isoWeek(new Date('2026-07-15T00:00:00Z'));
    assert.ok(w.week >= 1 && w.week <= 53);
    assert.equal(w.year, 2026);
  });
});

describe('weekStart', () => {
  it('returns Monday of the same week', () => {
    const mon = weekStart(new Date('2026-07-15T15:00:00'));
    assert.equal(mon.getDay(), 1); // Monday
    assert.equal(mon.getDate(), 13);
  });

  it('treats Sunday as the end of the week (Monday is 6 days prior)', () => {
    const mon = weekStart(new Date('2026-07-19T15:00:00'));
    assert.equal(mon.getDay(), 1);
    assert.equal(mon.getDate(), 13);
  });
});

describe('weeklyCounts', () => {
  it('returns numWeeks weeks, oldest first, last is current', () => {
    const weeks = weeklyCounts([], NOW, 8);
    assert.equal(weeks.length, 8);
    assert.equal(weeks[7].isCurrent, true);
    assert.equal(weeks[0].isCurrent, false);
    assert.equal(weeks.every(w => w.count === 0), true);
  });

  it('counts a job applied this week in the current week', () => {
    const jobs = [{ appliedDate: daysAgoISO(2) }];
    const weeks = weeklyCounts(jobs, NOW, 8);
    assert.equal(weeks[7].count, 1);
    assert.equal(weeks.slice(0, 7).every(w => w.count === 0), true);
  });

  it('counts a job applied 3 weeks ago in the right bucket', () => {
    const jobs = [{ appliedDate: daysAgoISO(21) }];
    const weeks = weeklyCounts(jobs, NOW, 8);
    assert.equal(weeks[4].count, 1); // weeks[7]=now, [6]=1wk, [5]=2wk, [4]=3wk
  });

  it('ignores jobs without appliedDate', () => {
    const jobs = [{ position: 'X' }, { appliedDate: null }, { appliedDate: '' }];
    const weeks = weeklyCounts(jobs, NOW, 8);
    assert.equal(weeks.every(w => w.count === 0), true);
  });

  it('ignores jobs with invalid appliedDate', () => {
    const jobs = [{ appliedDate: 'not-a-date' }];
    const weeks = weeklyCounts(jobs, NOW, 8);
    assert.equal(weeks.every(w => w.count === 0), true);
  });

  it('ignores jobs older than the window', () => {
    const jobs = [{ appliedDate: daysAgoISO(100) }];
    const weeks = weeklyCounts(jobs, NOW, 8);
    assert.equal(weeks.every(w => w.count === 0), true);
  });

  it('counts multiple jobs in the same week', () => {
    const jobs = [
      { appliedDate: daysAgoISO(2) },
      { appliedDate: daysAgoISO(4) },
      { appliedDate: daysAgoISO(5) },
    ];
    const weeks = weeklyCounts(jobs, NOW, 8);
    assert.equal(weeks[7].count, 3);
  });

  it('labels weeks with W prefix + ISO week number', () => {
    const weeks = weeklyCounts([], NOW, 8);
    assert.match(weeks[0].label, /^W\d+$/);
  });

  it('respects custom numWeeks', () => {
    const weeks = weeklyCounts([], NOW, 4);
    assert.equal(weeks.length, 4);
  });

  it('handles null jobs', () => {
    const weeks = weeklyCounts(null, NOW, 8);
    assert.equal(weeks.length, 8);
    assert.equal(weeks.every(w => w.count === 0), true);
  });
});

describe('weeklySummary', () => {
  it('sums counts and computes average', () => {
    const weeks = [
      { count: 1 }, { count: 3 }, { count: 0 }, { count: 2 },
      { count: 0 }, { count: 0 }, { count: 1 }, { count: 2 },
    ];
    const s = weeklySummary(weeks);
    assert.equal(s.total, 9);
    assert.equal(s.avg, 9 / 8);
  });

  it('handles empty weeks array', () => {
    const s = weeklySummary([]);
    assert.equal(s.total, 0);
    assert.equal(s.avg, 0);
  });
});
