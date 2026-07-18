import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { urgencyFor } from './urgency.js';

const NOW = new Date('2026-07-19T12:00:00Z');
function daysAgoISO(d) {
  const dt = new Date(NOW);
  dt.setDate(dt.getDate() - d);
  return dt.toISOString().slice(0, 10);
}
function daysAheadISO(d) {
  const dt = new Date(NOW);
  dt.setDate(dt.getDate() + d);
  return dt.toISOString().slice(0, 10);
}

describe('urgencyFor — stale', () => {
  it('flags Applied > 14 days ago as stale (amber at 15-21d)', () => {
    const j = { status: 'Applied', appliedDate: daysAgoISO(15) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'stale');
    assert.equal(u.tone, 'amber');
    assert.equal(u.label, '15d stale');
  });

  it('goes red at > 21 days', () => {
    const j = { status: 'Applied', appliedDate: daysAgoISO(22) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'stale');
    assert.equal(u.tone, 'red');
    assert.equal(u.label, '22d stale');
  });

  it('does NOT flag Applied exactly at the 14d boundary', () => {
    const j = { status: 'Applied', appliedDate: daysAgoISO(14) };
    const u = urgencyFor(j, NOW);
    assert.notEqual(u.kind, 'stale');
  });

  it('does NOT flag fresh Applied jobs', () => {
    const j = { status: 'Applied', appliedDate: daysAgoISO(3) };
    const u = urgencyFor(j, NOW);
    assert.notEqual(u.kind, 'stale');
  });

  it('does NOT flag stale days on non-Applied statuses', () => {
    const j = { status: 'Offer', appliedDate: daysAgoISO(40) };
    const u = urgencyFor(j, NOW);
    assert.notEqual(u.kind, 'stale');
  });
});

describe('urgencyFor — deadline', () => {
  it('shows days-left for upcoming deadlines ≤ 7 days', () => {
    const j = { status: 'Not Applied', date: daysAheadISO(5) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'deadline');
    assert.equal(u.tone, 'amber');
    assert.equal(u.label, '5d left');
  });

  it('uses "Today" at day 0', () => {
    const j = { status: 'Not Applied', date: daysAheadISO(0) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'deadline');
    assert.equal(u.label, 'Today');
  });

  it('shows overdue (red) for Not Applied with a passed deadline', () => {
    const j = { status: 'Not Applied', date: daysAgoISO(10) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'deadline');
    assert.equal(u.tone, 'red');
    assert.equal(u.label, '10d overdue');
  });

  it('does NOT treat Applied jobs with passed deadlines as overdue', () => {
    const j = { status: 'Applied', appliedDate: daysAgoISO(2), date: daysAgoISO(5) };
    const u = urgencyFor(j, NOW);
    assert.notEqual(u.kind, 'deadline');
  });

  it('falls through when deadline is > 7d out', () => {
    const j = { status: 'Not Applied', date: daysAheadISO(20) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'none');
  });
});

describe('urgencyFor — precedence', () => {
  it('stale wins over deadline', () => {
    // 20 days stale Applied job with an imminent deadline — stale is
    // the bottleneck (no response), so silence the deadline signal.
    const j = { status: 'Applied', appliedDate: daysAgoISO(20), date: daysAheadISO(3) };
    const u = urgencyFor(j, NOW);
    assert.equal(u.kind, 'stale');
  });
});

describe('urgencyFor — closed / offer / none', () => {
  it('Rejected → Closed', () => {
    assert.equal(urgencyFor({ status: 'Rejected' }, NOW).kind, 'closed');
    assert.equal(urgencyFor({ status: 'Rejected' }, NOW).label, 'Closed');
  });

  it('Withdrawn → Withdrawn', () => {
    assert.equal(urgencyFor({ status: 'Withdrawn' }, NOW).kind, 'closed');
    assert.equal(urgencyFor({ status: 'Withdrawn' }, NOW).label, 'Withdrawn');
  });

  it('Offer → No deadline', () => {
    const u = urgencyFor({ status: 'Offer' }, NOW);
    assert.equal(u.kind, 'offer');
    assert.equal(u.label, 'No deadline');
  });

  it('Not Applied with no date → none', () => {
    const u = urgencyFor({ status: 'Not Applied' }, NOW);
    assert.equal(u.kind, 'none');
    assert.equal(u.label, '—');
  });

  it('null job → none', () => {
    const u = urgencyFor(null, NOW);
    assert.equal(u.kind, 'none');
  });
});
