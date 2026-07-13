import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { formatDate, formatDateShort, formatDateTime, formatDateFull } from './format.js';

describe('formatDate', () => {
  it('formats a date with month, day, and year', () => {
    const result = formatDate('2026-01-15');
    assert.ok(result.includes('Jan'));
    assert.ok(result.includes('15'));
    assert.ok(result.includes('2026'));
  });

  it('returns empty string for null', () => {
    assert.equal(formatDate(null), '');
  });

  it('returns empty string for empty string', () => {
    assert.equal(formatDate(''), '');
  });

  it('returns empty string for undefined', () => {
    assert.equal(formatDate(undefined), '');
  });
});

describe('formatDateShort', () => {
  it('formats a date with month and day, no year', () => {
    const result = formatDateShort('2026-01-15');
    assert.ok(result.includes('Jan'));
    assert.ok(result.includes('15'));
    assert.ok(!result.includes('2026'));
  });

  it('returns empty string for null', () => {
    assert.equal(formatDateShort(null), '');
  });
});

describe('formatDateTime', () => {
  it('formats a date with month, day, and time, no year', () => {
    const result = formatDateTime('2026-01-15T10:30:00');
    assert.ok(result.includes('Jan'));
    assert.ok(result.includes('15'));
    assert.ok(!result.includes('2026'));
  });

  it('returns empty string for null', () => {
    assert.equal(formatDateTime(null), '');
  });
});

describe('formatDateFull', () => {
  it('formats a date with month, day, year, and time', () => {
    const result = formatDateFull('2026-01-15T10:30:00');
    assert.ok(result.includes('Jan'));
    assert.ok(result.includes('15'));
    assert.ok(result.includes('2026'));
  });

  it('returns empty string for null', () => {
    assert.equal(formatDateFull(null), '');
  });
});
