import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import {
  PROFILE_TABS,
  DEFAULT_PROFILE_TAB,
  normalizeTab,
  tabFromParams,
  resolveTab,
  saveTab,
} from './profileTabs.js';

describe('normalizeTab', () => {
  it('passes through valid tabs', () => {
    assert.equal(normalizeTab('profile'), 'profile');
    assert.equal(normalizeTab('preferences'), 'preferences');
  });

  it('falls back to default for invalid values', () => {
    assert.equal(normalizeTab('garbage'), DEFAULT_PROFILE_TAB);
    assert.equal(normalizeTab(''), DEFAULT_PROFILE_TAB);
    assert.equal(normalizeTab(null), DEFAULT_PROFILE_TAB);
    assert.equal(normalizeTab(undefined), DEFAULT_PROFILE_TAB);
    assert.equal(normalizeTab(42), DEFAULT_PROFILE_TAB);
  });
});

describe('tabFromParams', () => {
  it('reads the tab param', () => {
    const params = new URLSearchParams('tab=preferences');
    assert.equal(tabFromParams(params), 'preferences');
  });

  it('falls back to default when param is absent', () => {
    const params = new URLSearchParams('');
    assert.equal(tabFromParams(params), DEFAULT_PROFILE_TAB);
  });

  it('falls back to default when param is invalid', () => {
    const params = new URLSearchParams('tab=garbage');
    assert.equal(tabFromParams(params), DEFAULT_PROFILE_TAB);
  });

  it('tolerates null params', () => {
    assert.equal(tabFromParams(null), DEFAULT_PROFILE_TAB);
  });
});

describe('resolveTab', () => {
  // window.location is a Location object in the browser; the stub provides
  // the one property resolveTab reads (.search).
  const loc = (search) => ({ location: { search } });

  it('URL wins over storage when the param is present', () => {
    global.window = loc('?tab=preferences');
    global.localStorage = { getItem: () => 'profile' };
    assert.equal(resolveTab(), 'preferences');
    delete global.window;
    delete global.localStorage;
  });

  it('falls back to storage when the URL has no tab param', () => {
    global.window = loc('');
    global.localStorage = { getItem: () => 'preferences' };
    assert.equal(resolveTab(), 'preferences');
    delete global.window;
    delete global.localStorage;
  });

  it('falls back to default when neither has a value', () => {
    global.window = loc('');
    global.localStorage = { getItem: () => null };
    assert.equal(resolveTab(), DEFAULT_PROFILE_TAB);
    delete global.window;
    delete global.localStorage;
  });
});

describe('saveTab', () => {
  beforeEach(() => {
    global.window = { location: 'http://localhost/profile' };
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

  it('writes the tab to localStorage and returns the normalized value', () => {
    const result = saveTab('preferences');
    assert.equal(result, 'preferences');
    assert.equal(global.localStorage.getItem('waypoint_profile_tab'), 'preferences');
  });

  it('normalizes before writing', () => {
    const result = saveTab('garbage');
    assert.equal(result, DEFAULT_PROFILE_TAB);
    assert.equal(global.localStorage.getItem('waypoint_profile_tab'), DEFAULT_PROFILE_TAB);
  });

  it('exposes the canonical tab list', () => {
    assert.deepEqual(PROFILE_TABS, ['profile', 'preferences']);
  });
});
