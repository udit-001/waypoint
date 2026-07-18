import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import {
  LAYOUTS,
  DEFAULT_LAYOUT,
  normalizeLayout,
  layoutFromParams,
  saveLayout,
} from './layout.js';

describe('normalizeLayout', () => {
  it('passes through valid layouts', () => {
    assert.equal(normalizeLayout('list'), 'list');
    assert.equal(normalizeLayout('kanban'), 'kanban');
  });

  it('falls back to default for invalid values', () => {
    assert.equal(normalizeLayout('garbage'), DEFAULT_LAYOUT);
    assert.equal(normalizeLayout(''), DEFAULT_LAYOUT);
    assert.equal(normalizeLayout(null), DEFAULT_LAYOUT);
    assert.equal(normalizeLayout(undefined), DEFAULT_LAYOUT);
    assert.equal(normalizeLayout(42), DEFAULT_LAYOUT);
  });
});

describe('layoutFromParams', () => {
  it('reads the layout param', () => {
    const params = new URLSearchParams('layout=kanban');
    assert.equal(layoutFromParams(params), 'kanban');
  });

  it('falls back to default when param is absent', () => {
    const params = new URLSearchParams('');
    assert.equal(layoutFromParams(params), DEFAULT_LAYOUT);
  });

  it('falls back to default when param is invalid', () => {
    const params = new URLSearchParams('layout=garbage');
    assert.equal(layoutFromParams(params), DEFAULT_LAYOUT);
  });

  it('tolerates null params', () => {
    assert.equal(layoutFromParams(null), DEFAULT_LAYOUT);
  });
});

describe('saveLayout', () => {
  beforeEach(() => {
    // jsdom-ish stubs: we only need URL + history.replaceState + localStorage.
    // `window.location` must be a real URL string — `new URL(window.location)`
    // in saveLayout requires an absolute href.
    global.window = { location: 'http://localhost/applications' };
    global.history = { replaceState: () => {} };
    const store = new Map();
    global.localStorage = {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => store.set(k, String(v)),
      removeItem: (k) => store.delete(k),
    };
  });
  afterEach(() => {
    delete global.window;
    delete global.history;
    delete global.localStorage;
  });

  it('writes the layout to localStorage and returns the normalized value', () => {
    saveLayout('kanban');
    assert.equal(global.localStorage.getItem('waypoint_applications_layout'), 'kanban');
  });

  it('normalizes before writing', () => {
    const result = saveLayout('garbage');
    assert.equal(result, DEFAULT_LAYOUT);
    assert.equal(global.localStorage.getItem('waypoint_applications_layout'), DEFAULT_LAYOUT);
  });

  it('exposes the canonical layout list', () => {
    assert.deepEqual(LAYOUTS, ['list', 'kanban']);
  });
});
