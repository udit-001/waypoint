import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { parseSalary } from './salary.js';

describe('parseSalary — monthly salaries', () => {
  it('parses Indian Rs. format with HRA as monthly', () => {
    assert.deepEqual(parseSalary('Rs. 37,000 + HRA'), { low: 37, high: 37, mid: 37, currency: '₹' });
  });

  it('parses ₹ symbol as monthly', () => {
    assert.deepEqual(parseSalary('₹50000'), { low: 50, high: 50, mid: 50, currency: '₹' });
  });

  it('parses Indian comma-format number as monthly', () => {
    assert.deepEqual(parseSalary('37,000'), { low: 37, high: 37, mid: 37, currency: '₹' });
  });
});

describe('parseSalary — annual salaries', () => {
  it('parses $ k-format as annual, normalises to monthly', () => {
    assert.deepEqual(parseSalary('$120k'), { low: 10, high: 10, mid: 10, currency: '$' });
  });

  it('parses k-format range as annual', () => {
    assert.deepEqual(parseSalary('160k-200k'), { low: 13, high: 17, mid: 15, currency: null });
  });

  it('parses bare number as annual (default)', () => {
    assert.deepEqual(parseSalary('50000'), { low: 4, high: 4, mid: 4, currency: null });
  });
});

describe('parseSalary — lakh magnitude (bug fix)', () => {
  it('parses single LPA with correct magnitude', () => {
    assert.deepEqual(parseSalary('12 LPA'), { low: 100, high: 100, mid: 100, currency: '₹' });
  });

  it('parses LPA range with correct magnitude', () => {
    assert.deepEqual(parseSalary('12-15 LPA'), { low: 100, high: 125, mid: 113, currency: '₹' });
  });

  it('parses "lakh" keyword with correct magnitude', () => {
    assert.deepEqual(parseSalary('8 lakh'), { low: 67, high: 67, mid: 67, currency: '₹' });
  });

  it('parses Rs. with LPA', () => {
    assert.deepEqual(parseSalary('Rs. 12 LPA'), { low: 100, high: 100, mid: 100, currency: '₹' });
  });

  it('does NOT treat bare "PA" as lakh (period only, no multiplier)', () => {
    const result = parseSalary('12 PA');
    assert.equal(result.mid, 1);
    assert.equal(result.currency, null);
  });
});

describe('parseSalary — currency detection (bug fix)', () => {
  it('returns $ for dollar salaries, not ₹', () => {
    const result = parseSalary('$100k');
    assert.equal(result.currency, '$');
    assert.notEqual(result.currency, '₹');
  });

  it('returns null for bare numbers with no currency signal', () => {
    assert.equal(parseSalary('60000').currency, null);
  });

  it('returns ₹ for LPA', () => {
    assert.equal(parseSalary('10 LPA').currency, '₹');
  });
});

describe('parseSalary — OR alternatives', () => {
  it('picks the best (highest mid) option, shows full range', () => {
    assert.deepEqual(
      parseSalary('Rs. 37,000 + HRA OR Rs. 31,000 + HRA'),
      { low: 31, high: 37, mid: 37, currency: '₹' }
    );
  });

  it('returns null when all alternatives fail to parse', () => {
    assert.equal(parseSalary('abc OR def'), null);
  });
});

describe('parseSalary — edge cases', () => {
  it('returns null for empty string', () => {
    assert.equal(parseSalary(''), null);
  });

  it('returns null for null input', () => {
    assert.equal(parseSalary(null), null);
  });

  it('returns null for garbage with no numbers', () => {
    assert.equal(parseSalary('garbage'), null);
  });

  it('returns null for whitespace-only', () => {
    assert.equal(parseSalary('   '), null);
  });

  it('handles reversed range (low > high)', () => {
    const result = parseSalary('35000-25000');
    assert.ok(result.low <= result.high, 'low should not exceed high');
  });
});
