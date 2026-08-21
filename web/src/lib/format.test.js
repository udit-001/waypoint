import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { formatDate, formatDateShort, formatDateTime, formatDateFull, formatMonth, relTime } from './format.js';

describe('formatDate', () => {
  it('formats a date with month, day, and year', () => {
    const result = formatDate('2026-01-15');
    assert.ok(result.includes('Jan'));
    assert.ok(result.includes('15'));
    assert.ok(result.includes('2026'));
  });

  it('returns unparsable input unchanged — never "Invalid Date"', () => {
    // Scraper dates are hostile: new Date(garbage) does not throw, it
    // renders as "Invalid Date" via toLocale*. The guard must return
    // the original string for the caller to fall back on.
    assert.equal(formatDate('not-a-date'), 'not-a-date');
    assert.equal(formatDate('2026-13-99'), '2026-13-99');
    assert.equal(formatDateShort('garbage'), 'garbage');
    assert.equal(formatDateTime('garbage'), 'garbage');
    assert.equal(formatDateFull('garbage'), 'garbage');
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

describe('relTime', () => {
  // Fixed reference: 2026-08-22T12:00:00Z.
  const NOW = Date.parse('2026-08-22T12:00:00Z');
  const ago = (iso) => relTime(iso, NOW);

  it('labels sub-minute age as "just now"', () => {
    assert.equal(ago('2026-08-22T11:59:40Z'), 'just now');
  });

  it('renders minutes, hours, and days', () => {
    assert.equal(ago('2026-08-22T11:58:00Z'), '2m ago');
    assert.equal(ago('2026-08-22T09:00:00Z'), '3h ago');
    assert.equal(ago('2026-08-20T12:00:00Z'), '2d ago');
  });

  it('falls back to a short date past a month', () => {
    const out = ago('2026-06-01T12:00:00Z');
    assert.ok(out.includes('Jun'), `expected month name in ${out}`);
  });

  it('returns empty string for empty/null input', () => {
    assert.equal(relTime('', NOW), '');
    assert.equal(relTime(null, NOW), '');
  });

  it('returns unparsable input unchanged — never "Invalid Date"', () => {
    assert.equal(relTime('garbage', NOW), 'garbage');
  });
});
