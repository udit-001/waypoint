import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { applyFilter, bucketFor, isStale, matchesText } from './filter.js';

const today = new Date();
function daysFromNow(d) {
  const dt = new Date(today);
  dt.setDate(dt.getDate() + d);
  return dt.toISOString().slice(0, 10);
}
const staleApplied = { id: 1, company: 'Google', position: 'SWE', status: 'Applied', category: 'Tech', appliedDate: daysFromNow(-20) };
const freshApplied = { id: 2, company: 'Meta', position: 'SWE', status: 'Applied', category: 'Tech', appliedDate: daysFromNow(-3) };
const offerTech = { id: 3, company: 'Acme', position: 'SWE', status: 'Offer', category: 'Tech', appliedDate: daysFromNow(-10) };
const rejectedBio = { id: 4, company: 'Lab', position: 'Sci', status: 'Rejected', category: 'Biotech', appliedDate: daysFromNow(-5) };
const notAppliedWithDeadline = { id: 5, company: 'Init', position: 'Dev', status: 'Not Applied', category: 'Tech', date: daysFromNow(5) };
const notAppliedOverdue = { id: 6, company: 'Old', position: 'Dev', status: 'Not Applied', category: 'Finance', date: daysFromNow(-10) };
const appliedOverdue = { id: 7, company: 'Late', position: 'Dev', status: 'Applied', category: 'Finance', date: daysFromNow(-10), appliedDate: daysFromNow(-12) };
const noDate = { id: 8, company: 'Mystery', position: 'Dev', status: 'Not Applied', category: 'Tech' };

const jobs = [staleApplied, freshApplied, offerTech, rejectedBio, notAppliedWithDeadline, notAppliedOverdue, appliedOverdue, noDate];

describe('applyFilter — multi-value category + status', () => {
  it('returns all jobs when no filter is active', () => {
    assert.equal(applyFilter(jobs, {}).length, 8);
  });

  it('returns all jobs when filter arrays are empty', () => {
    assert.equal(applyFilter(jobs, { categories: [], statuses: [] }).length, 8);
  });

  it('filters by a single category', () => {
    const result = applyFilter(jobs, { categories: ['Tech'] });
    assert.equal(result.length, 5);
    assert.ok(result.every(j => j.category === 'Tech'));
  });

  it('filters by multiple categories (OR)', () => {
    const result = applyFilter(jobs, { categories: ['Tech', 'Finance'] });
    assert.equal(result.length, 7);
    assert.ok(result.every(j => ['Tech', 'Finance'].includes(j.category)));
  });

  it('filters by a single status', () => {
    const result = applyFilter(jobs, { statuses: ['Applied'] });
    assert.equal(result.length, 3);
    assert.ok(result.every(j => j.status === 'Applied'));
  });

  it('filters by multiple statuses (OR)', () => {
    const result = applyFilter(jobs, { statuses: ['Applied', 'Offer'] });
    assert.equal(result.length, 4);
    assert.ok(result.every(j => ['Applied', 'Offer'].includes(j.status)));
  });

  it('combines category + status (AND across dimensions, OR within)', () => {
    const result = applyFilter(jobs, { categories: ['Tech'], statuses: ['Applied', 'Offer'] });
    assert.equal(result.length, 3);
    assert.ok(result.every(j => j.category === 'Tech' && ['Applied', 'Offer'].includes(j.status)));
  });

  it('returns empty array when no jobs match', () => {
    assert.equal(applyFilter(jobs, { categories: ['Nonexistent'] }).length, 0);
  });

  it('filters to uncategorized jobs (empty-string category matches missing category)', () => {
    const noCat = { id: 99, company: 'NoCat', position: 'X', status: 'Applied' }; // no category field
    const result = applyFilter([...jobs, noCat], { categories: [''] });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 99);
  });

  it('combines uncategorized + a named category (OR within dimension)', () => {
    const noCat = { id: 99, company: 'NoCat', position: 'X', status: 'Applied' };
    const result = applyFilter([...jobs, noCat], { categories: ['Tech', ''] });
    assert.ok(result.find(j => j.id === 99)); // uncategorized
    assert.ok(result.find(j => j.id === 1));  // Tech
  });

  it('handles null jobs', () => {
    assert.deepEqual(applyFilter(null, { categories: ['Tech'] }), []);
  });

  it('handles undefined jobs', () => {
    assert.deepEqual(applyFilter(undefined, { categories: ['Tech'] }), []);
  });

  it('does not mutate the input array', () => {
    const original = [...jobs];
    applyFilter(jobs, { categories: ['Tech'] });
    assert.deepEqual(jobs, original);
  });
});

describe('applyFilter — deadline bucket (single-select, mutually exclusive)', () => {
  it('overdue = deadline passed AND status = Not Applied', () => {
    const result = applyFilter(jobs, { deadlineBucket: 'overdue' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 6);
  });

  it('overdue excludes Applied jobs even if deadline passed (deadline served its purpose)', () => {
    const result = applyFilter(jobs, { deadlineBucket: 'overdue' });
    assert.ok(!result.find(j => j.id === 7)); // appliedOverdue should NOT match
  });

  it('this-week = deadline in next 7 days', () => {
    const result = applyFilter(jobs, { deadlineBucket: 'this-week' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 5);
  });

  it('this-month = deadline in 8-30 days (excludes this-week)', () => {
    const farFuture = { id: 9, company: 'Far', position: 'X', status: 'Not Applied', category: 'Tech', date: daysFromNow(20) };
    const result = applyFilter([...jobs, farFuture], { deadlineBucket: 'this-month' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 9);
  });

  it('no-date = date field empty', () => {
    const result = applyFilter(jobs, { deadlineBucket: 'no-date' });
    assert.equal(result.length, 5); // staleApplied, freshApplied, offerTech, rejectedBio, noDate
    assert.ok(result.every(j => !j.date));
  });

  it('deadline > 30 days away matches no bucket', () => {
    const farFuture = { id: 9, company: 'Far', position: 'X', status: 'Not Applied', category: 'Tech', date: daysFromNow(45) };
    assert.equal(applyFilter([farFuture], { deadlineBucket: 'this-month' }).length, 0);
    assert.equal(applyFilter([farFuture], { deadlineBucket: 'no-date' }).length, 0);
  });
});

describe('applyFilter — stale toggle', () => {
  it('stale = Applied AND appliedDate > 14 days ago', () => {
    const result = applyFilter(jobs, { stale: true });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 1); // staleApplied (20d ago)
  });

  it('stale excludes fresh Applied (<=14d)', () => {
    const result = applyFilter(jobs, { stale: true });
    assert.ok(!result.find(j => j.id === 2)); // freshApplied (3d ago)
  });

  it('stale excludes Offer/Rejected/Withdrawn (got a response)', () => {
    const result = applyFilter(jobs, { stale: true });
    assert.ok(!result.find(j => j.id === 3)); // Offer
    assert.ok(!result.find(j => j.id === 4)); // Rejected
  });

  it('stale=false does NOT filter (shows all)', () => {
    assert.equal(applyFilter(jobs, { stale: false }).length, 8);
  });
});

describe('applyFilter — text-query (substring over text fields)', () => {
  it('matches company name (case-insensitive)', () => {
    const result = applyFilter(jobs, { textQuery: 'goo' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 1);
  });

  it('matches position', () => {
    const result = applyFilter(jobs, { textQuery: 'swe' });
    assert.equal(result.length, 3); // Google, Meta, Acme
  });

  it('matches category', () => {
    const result = applyFilter(jobs, { textQuery: 'bio' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 4);
  });

  it('matches location if present', () => {
    const withLoc = [...jobs, { id: 99, company: 'X', position: 'Y', status: 'Applied', category: 'Tech', location: 'Bangalore' }];
    const result = applyFilter(withLoc, { textQuery: 'bang' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 99);
  });

  it('matches notes if present', () => {
    const withNotes = [...jobs, { id: 99, company: 'X', position: 'Y', status: 'Applied', category: 'Tech', notes: 'referral from jane' }];
    const result = applyFilter(withNotes, { textQuery: 'jane' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 99);
  });

  it('trims whitespace before matching', () => {
    assert.equal(applyFilter(jobs, { textQuery: '  goo  ' }).length, 1);
  });

  it('empty text-query does not filter', () => {
    assert.equal(applyFilter(jobs, { textQuery: '' }).length, 8);
    assert.equal(applyFilter(jobs, { textQuery: '   ' }).length, 8);
  });
});

describe('applyFilter — dimension combinations (AND across dims)', () => {
  it('category + stale', () => {
    const result = applyFilter(jobs, { categories: ['Tech'], stale: true });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 1);
  });

  it('status + deadline bucket', () => {
    const result = applyFilter(jobs, { statuses: ['Not Applied'], deadlineBucket: 'overdue' });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 6);
  });

  it('text-query + category', () => {
    const result = applyFilter(jobs, { categories: ['Tech'], textQuery: 'swe' });
    assert.equal(result.length, 3); // Google, Meta, Acme (all Tech + SWE)
  });

  it('all 5 dimensions at once', () => {
    const result = applyFilter(jobs, {
      categories: ['Tech'],
      statuses: ['Applied'],
      stale: true,
      textQuery: 'goo',
    });
    assert.equal(result.length, 1);
    assert.equal(result[0].id, 1);
  });
});

describe('bucketFor', () => {
  it('returns no-date when date is missing', () => {
    assert.equal(bucketFor({ status: 'Not Applied' }), 'no-date');
  });

  it('returns overdue when deadline passed and Not Applied', () => {
    assert.equal(bucketFor({ status: 'Not Applied', date: daysFromNow(-1) }), 'overdue');
  });

  it('returns null when deadline passed but Applied (deadline served purpose)', () => {
    assert.equal(bucketFor({ status: 'Applied', date: daysFromNow(-1) }), null);
  });

  it('returns this-week for 0-7 days', () => {
    assert.equal(bucketFor({ status: 'Not Applied', date: daysFromNow(0) }), 'this-week');
    assert.equal(bucketFor({ status: 'Not Applied', date: daysFromNow(7) }), 'this-week');
  });

  it('returns this-month for 8-30 days', () => {
    assert.equal(bucketFor({ status: 'Not Applied', date: daysFromNow(8) }), 'this-month');
    assert.equal(bucketFor({ status: 'Not Applied', date: daysFromNow(30) }), 'this-month');
  });

  it('returns null for >30 days', () => {
    assert.equal(bucketFor({ status: 'Not Applied', date: daysFromNow(31) }), null);
  });
});

describe('isStale', () => {
  it('true for Applied >14d ago', () => {
    assert.equal(isStale({ status: 'Applied', appliedDate: daysFromNow(-15) }), true);
    assert.equal(isStale({ status: 'Applied', appliedDate: daysFromNow(-20) }), true);
  });

  it('false for Applied <=14d ago', () => {
    assert.equal(isStale({ status: 'Applied', appliedDate: daysFromNow(-14) }), false);
    assert.equal(isStale({ status: 'Applied', appliedDate: daysFromNow(-3) }), false);
  });

  it('false for non-Applied statuses (got a response)', () => {
    assert.equal(isStale({ status: 'Offer', appliedDate: daysFromNow(-30) }), false);
    assert.equal(isStale({ status: 'Rejected', appliedDate: daysFromNow(-30) }), false);
    assert.equal(isStale({ status: 'Withdrawn', appliedDate: daysFromNow(-30) }), false);
  });

  it('false for Applied with no appliedDate', () => {
    assert.equal(isStale({ status: 'Applied' }), false);
  });
});

describe('matchesText', () => {
  it('case-insensitive company match', () => {
    assert.equal(matchesText({ company: 'Google' }, 'GOO'), true);
  });

  it('matches position', () => {
    assert.equal(matchesText({ position: 'Engineer' }, 'gin'), true);
  });

  it('matches category', () => {
    assert.equal(matchesText({ category: 'Biotech' }, 'bio'), true);
  });

  it('returns false when no field matches', () => {
    assert.equal(matchesText({ company: 'Google', position: 'SWE' }, 'acme'), false);
  });

  it('returns false for empty query', () => {
    assert.equal(matchesText({ company: 'Google' }, ''), false);
  });
});
