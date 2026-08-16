import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { formatDate, formatDateShort, formatDateTime, formatDateFull, formatMonth } from './format.js';

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

describe('formatMonth', () => {
  it('formats partial ISO YYYY-MM as abbreviated month + year', () => {
    assert.equal(formatMonth('2023-03'), 'Mar 2023');
  });

  it('formats December and January boundaries', () => {
    assert.equal(formatMonth('2000-01'), 'Jan 2000');
    assert.equal(formatMonth('1999-12'), 'Dec 1999');
  });

  it('returns empty string for empty input', () => {
    assert.equal(formatMonth(''), '');
    assert.equal(formatMonth(null), '');
    assert.equal(formatMonth(undefined), '');
  });

  it('returns invalid input unchanged', () => {
    assert.equal(formatMonth('2023-13'), '2023-13');
    assert.equal(formatMonth('not-a-date'), 'not-a-date');
  });
});
