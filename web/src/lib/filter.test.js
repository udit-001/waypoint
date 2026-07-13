import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { applyFilter } from './filter.js';

const jobs = [
  { id: 1, company: 'Google', status: 'Applied', category: 'Tech' },
  { id: 2, company: 'Meta', status: 'Offer', category: 'Tech' },
  { id: 3, company: 'Startup', status: 'Applied', category: 'Biotech' },
  { id: 4, company: 'Lab', status: 'Rejected', category: 'Biotech' },
];

describe('applyFilter', () => {
  it('returns all jobs when no filter is active', () => {
    assert.equal(applyFilter(jobs, {}).length, 4);
  });

  it('returns all jobs when filter values are empty strings', () => {
    assert.equal(applyFilter(jobs, { category: '', status: '' }).length, 4);
  });

  it('filters by category', () => {
    const result = applyFilter(jobs, { category: 'Tech', status: '' });
    assert.equal(result.length, 2);
    assert.ok(result.every(j => j.category === 'Tech'));
  });

  it('filters by status', () => {
    const result = applyFilter(jobs, { category: '', status: 'Applied' });
    assert.equal(result.length, 2);
    assert.ok(result.every(j => j.status === 'Applied'));
  });

  it('filters by both category and status', () => {
    const result = applyFilter(jobs, { category: 'Tech', status: 'Applied' });
    assert.equal(result.length, 1);
    assert.equal(result[0].company, 'Google');
  });

  it('returns empty array when no jobs match', () => {
    assert.equal(applyFilter(jobs, { category: 'Finance', status: '' }).length, 0);
  });

  it('handles null jobs', () => {
    assert.deepEqual(applyFilter(null, { category: 'Tech', status: '' }), []);
  });

  it('handles undefined jobs', () => {
    assert.deepEqual(applyFilter(undefined, { category: 'Tech', status: '' }), []);
  });

  it('does not mutate the input array', () => {
    const original = [...jobs];
    applyFilter(jobs, { category: 'Tech', status: '' });
    assert.deepEqual(jobs, original);
  });
});
