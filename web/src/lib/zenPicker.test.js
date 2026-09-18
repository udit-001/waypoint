import { test } from 'node:test';
import assert from 'node:assert/strict';
import { DefaultZenModel, zenOptions } from './zenPicker.js';

const catalog = [
  { id: 'big-pickle', name: 'Big Pickle', api: 'openai-completions' },
  { id: 'union-alpha-free', name: 'Union Alpha Free', api: 'anthropic-messages' },
  { id: 'mimo-v2.5-free', name: 'MiMo V2.5 Free' },
];

test('leads with the shipped default option', () => {
  const opts = zenOptions(catalog, '');
  assert.deepEqual(opts[0], { value: '', label: 'Default (big-pickle)' });
  assert.equal(DefaultZenModel, 'big-pickle');
});

test('renders friendly names with family badge suffixes', () => {
  const opts = zenOptions(catalog, '').filter((o) => o.value);
  assert.deepEqual(opts, [
    { value: 'big-pickle', label: 'Big Pickle · chat' },
    { value: 'union-alpha-free', label: 'Union Alpha Free · anthropic' },
    { value: 'mimo-v2.5-free', label: 'MiMo V2.5 Free' }, // unknown api → no badge
  ]);
});

test('keeps a saved pick visible when it left the catalog', () => {
  const opts = zenOptions(catalog, 'hy3-free');
  assert.deepEqual(opts.at(-1), { value: 'hy3-free', label: 'hy3-free' });
});

test('does not duplicate the saved pick when it is in the catalog', () => {
  const opts = zenOptions(catalog, 'big-pickle');
  assert.equal(opts.filter((o) => o.value === 'big-pickle').length, 1);
});

test('degrades gracefully with an empty catalog', () => {
  assert.deepEqual(zenOptions([], ''), [{ value: '', label: 'Default (big-pickle)' }]);
  assert.deepEqual(zenOptions(undefined, 'saved-id').at(-1), { value: 'saved-id', label: 'saved-id' });
});
