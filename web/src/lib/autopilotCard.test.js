import { test } from 'node:test';
import assert from 'node:assert/strict';
import { chipState, verdictMeta, dotColor, shortWhen, CHIP_TONE, DOT_TONE } from './autopilotCard.js';

// WP-183: the state chip answers "does anything need me?" in one chip.
// waiting-on-you wins (people over broken machines), then degraded.
test('chipState: waiting-on-you is amber with the pending count', () => {
  assert.deepEqual(chipState('waiting-on-you', 3), { tone: 'amber', copy: '3 postings waiting' });
});

test('chipState: degraded is red with an empty queue', () => {
  assert.deepEqual(chipState('degraded', 0), { tone: 'red', copy: 'degraded' });
});

test('chipState: waiting beats degraded — people over machines', () => {
  assert.deepEqual(chipState('degraded', 2), { tone: 'amber', copy: '2 postings waiting' });
});

test('chipState: pending queue reads amber even after a quiet run', () => {
  assert.deepEqual(chipState('quiet', 3), { tone: 'amber', copy: '3 postings waiting' });
  assert.deepEqual(chipState('nothing-survived', 1), { tone: 'amber', copy: '1 posting waiting' });
});

// Tone→class maps live here, next to the code that mints the tones —
// the component only renders what this module names.
test('CHIP_TONE/DOT_TONE cover every tone the lib mints', () => {
  for (const t of ['slate', 'blue', 'green', 'amber', 'red']) assert.ok(CHIP_TONE[t], `CHIP_TONE missing ${t}`);
  for (const v of ['quiet', 'waiting-on-you', 'nothing-survived', 'degraded', '']) {
    assert.ok(DOT_TONE[dotColor(v)], `DOT_TONE missing ${dotColor(v)}`);
  }
});

test('chipState: quiet and nothing-survived are green', () => {
  for (const v of ['quiet', 'nothing-survived']) {
    assert.deepEqual(chipState(v, 0), { tone: 'green', copy: 'nothing needs you' });
  }
});

test('chipState: no run yet is green', () => {
  assert.deepEqual(chipState('', 0), { tone: 'green', copy: 'nothing needs you' });
});

// Verdict line copy — the artifact's fragment shapes.
test('verdictMeta: quiet states the count and outcome', () => {
  assert.deepEqual(
    verdictMeta({ verdict: 'quiet', postingsNew: 42 }),
    { chipClass: '', label: 'Quiet', what: '42 postings, none fit' },
  );
});

test('verdictMeta: nothing-survived counts the dismissals', () => {
  assert.deepEqual(
    verdictMeta({ verdict: 'nothing-survived', postingsNew: 17, postingsDismissed: 17 }),
    { chipClass: 'blue', label: 'Nothing survived', what: '17 postings, all dismissed' },
  );
});

test('verdictMeta: waiting-on-you names the queue', () => {
  assert.deepEqual(
    verdictMeta({ verdict: 'waiting-on-you', postingsShortlisted: 3 }),
    { chipClass: 'amber', label: 'Needs you', what: '3 shortlists waiting' },
  );
});

test('verdictMeta: degraded leads with the first stage error', () => {
  assert.deepEqual(
    verdictMeta({ verdict: 'degraded', stageErrors: ['sweep ncbs: timeout'] }),
    { chipClass: 'red', label: 'Degraded', what: 'sweep ncbs: timeout' },
  );
});

test('verdictMeta: running (empty verdict) reads as running', () => {
  assert.deepEqual(
    verdictMeta({ verdict: '' }),
    { chipClass: '', label: 'Running', what: 'this page updates when it finishes' },
  );
});

// Run dot colors: color = state only.
test('dotColor maps each verdict', () => {
  assert.equal(dotColor('quiet'), 'faint');
  assert.equal(dotColor('waiting-on-you'), 'amber');
  assert.equal(dotColor('nothing-survived'), 'blue');
  assert.equal(dotColor('degraded'), 'red');
  assert.equal(dotColor(''), 'faint');
});

// shortWhen: the artifact's "Today 06:12 / Yesterday / Mon" forms.
const NOW = new Date('2026-10-19T15:00:00');

test('shortWhen: today shows the time', () => {
  assert.equal(shortWhen('2026-10-19T06:12:00', NOW), 'Today 06:12');
});

test('shortWhen: yesterday shows the word', () => {
  assert.equal(shortWhen('2026-10-18T09:00:00', NOW), 'Yesterday');
});

test('shortWhen: older shows the weekday', () => {
  assert.equal(shortWhen('2026-10-12T09:00:00', NOW), 'Mon');
});
