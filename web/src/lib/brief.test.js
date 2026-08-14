import { test } from 'node:test';
import assert from 'node:assert/strict';
import { prettify, prettifyList } from './brief.js';

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
