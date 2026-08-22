import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { candidateBoardLabel, pendingCandidates } from './discovery.js';

describe('candidateBoardLabel', () => {
  it('renders provider with the token that identifies the board', () => {
    assert.equal(
      candidateBoardLabel({ provider: 'greenhouse', url: 'https://boards.greenhouse.io/acme' }),
      'greenhouse/acme'
    );
    assert.equal(
      candidateBoardLabel({ provider: 'workday', url: 'https://factset.wd1.myworkdayjobs.com/FactSetCareers/' }),
      'workday/FactSetCareers'
    );
    assert.equal(
      candidateBoardLabel({ provider: 'eightfold', url: 'https://citi.eightfold.ai/careers' }),
      'eightfold/citi'
    );
  });

  it('degrades gracefully on a malformed URL', () => {
    assert.equal(candidateBoardLabel({ provider: 'lever', url: 'not-a-url' }), 'lever/');
  });
});

describe('pendingCandidates', () => {
  it('keeps only suggested rows and tolerates null input', () => {
    const cands = [
      { id: 1, name: 'A', status: 'suggested' },
      { id: 2, name: 'B', status: 'dismissed' },
      { id: 3, name: 'C', status: 'added' },
    ];
    assert.deepEqual(pendingCandidates(cands).map(c => c.name), ['A']);
    assert.deepEqual(pendingCandidates(null), []);
  });
});
