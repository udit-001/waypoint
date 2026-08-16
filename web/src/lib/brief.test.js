import { test } from 'node:test';
import assert from 'node:assert/strict';
import { prettify, prettifyList, briefStatus } from './brief.js';

test('prettify sentence-cases a stored match value', () => {
  assert.equal(prettify('golang'), 'Golang');
  assert.equal(prettify('javascript'), 'Javascript');
  assert.equal(prettify('bengaluru'), 'Bengaluru');
  assert.equal(prettify('remote'), 'Remote');
  assert.equal(prettify(''), '');
  assert.equal(prettify(null), '');
});

test('prettifyList maps prettify over an array', () => {
  assert.deepEqual(prettifyList(['golang', 'acme']), ['Golang', 'Acme']);
  assert.deepEqual(prettifyList([]), []);
  assert.deepEqual(prettifyList(null), []);
});

test('briefStatus reports a complete brief as ready', () => {
  const s = briefStatus({ complete: true, open: [] });
  assert.equal(s.complete, true);
  assert.equal(s.openCount, 0);
  assert.equal(s.label, 'All set — ready to search.');
});

test('briefStatus counts open preferences with plural label', () => {
  const s = briefStatus({ complete: false, open: ['companies', 'keywords'] });
  assert.equal(s.complete, false);
  assert.equal(s.openCount, 2);
  assert.equal(s.label, '2 preferences still to set.');
});

test('briefStatus uses singular label for one open preference', () => {
  const s = briefStatus({ complete: false, open: ['remote'] });
  assert.equal(s.openCount, 1);
  assert.equal(s.label, '1 preference still to set.');
});

test('briefStatus tolerates a missing open list', () => {
  const s = briefStatus({ complete: false });
  assert.equal(s.openCount, 0);
  assert.equal(s.label, '0 preferences still to set.');
});
