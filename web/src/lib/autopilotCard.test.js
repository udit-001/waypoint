import { test } from 'node:test';
import assert from 'node:assert/strict';
import { chipState, verdictMeta, verdictSentence, dotColor, shortWhen, scheduleStatus, clockTime, progressLine, CHIP_TONE, DOT_TONE } from './autopilotCard.js';

// scheduleStatus: the autopilot progress line's three states + null.
test('scheduleStatus: the three progress states derive from the payload', () => {
  assert.deepEqual(scheduleStatus({ enabled: true, running: true }), { kind: 'run-in-progress' });
  assert.deepEqual(scheduleStatus({ enabled: true, firstRun: true }), { kind: 'first-run' });
  const scheduled = scheduleStatus({ enabled: true, nextRunAt: '2099-01-01T10:30:00Z' }, new Date('2099-01-01T09:00:00Z'));
  assert.equal(scheduled.kind, 'cadence-wait');
  assert.equal(scheduled.at.toISOString(), '2099-01-01T10:30:00.000Z');
});

test('scheduleStatus: disabled-suffixed states are null — off says nothing', () => {
  assert.equal(scheduleStatus(null), null);
  assert.equal(scheduleStatus({ enabled: false, firstRun: true }), null);
  assert.equal(scheduleStatus({ enabled: true }), null); // nothing known
});

test('scheduleStatus: a past nextRunAt is due — first-run urgency, not a stale clock', () => {
  assert.deepEqual(
    scheduleStatus({ enabled: true, nextRunAt: '2099-01-01T10:30:00Z' }, new Date('2099-01-01T11:00:00Z')).kind,
    'first-run',
  );
});

test('clockTime: brief clock form, null-safe', () => {
  assert.ok(/^\d{2}:\d{2}$/.test(clockTime(new Date('2099-01-01T10:30:00Z'))));
  assert.equal(clockTime(null), '');
});

// WP-183: the state chip answers "does anything need me?" in one chip.
// waiting-on-you wins (people over broken machines), then degraded.
// Every enabled-state test passes true: only the green idle tone is
// state-gated (the chip's own decision — see lib).
test('chipState: waiting-on-you is amber with the pending count', () => {
  assert.deepEqual(chipState(true, 'waiting-on-you', 3), { tone: 'amber', copy: '3 postings waiting' });
});

test('chipState: degraded is red with an empty queue', () => {
  assert.deepEqual(chipState(true, 'degraded', 0), { tone: 'red', copy: 'degraded' });
});

test('chipState: degraded shows even when disabled — a broken run stays broken until someone looks', () => {
  assert.deepEqual(chipState(false, 'degraded', 0), { tone: 'red', copy: 'degraded' });
});

test('chipState: waiting beats degraded — people over machines', () => {
  assert.deepEqual(chipState(true, 'degraded', 2), { tone: 'amber', copy: '2 postings waiting' });
});

test('chipState: waiting shows even when disabled — shortlists land while autopilot is off', () => {
  assert.deepEqual(chipState(false, 'waiting-on-you', 2), { tone: 'amber', copy: '2 postings waiting' });
});

test('chipState: pending queue reads amber even after a quiet run', () => {
  assert.deepEqual(chipState(true, 'quiet', 3), { tone: 'amber', copy: '3 postings waiting' });
  assert.deepEqual(chipState(true, 'nothing-survived', 1), { tone: 'amber', copy: '1 posting waiting' });
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
    assert.deepEqual(chipState(true, v, 0), { tone: 'green', copy: 'nothing needs you' });
  }
});

test('chipState: enabled with no run yet is green (first run still warming up)', () => {
  assert.deepEqual(chipState(true, '', 0), { tone: 'green', copy: 'nothing needs you' });
});

test('chipState: disabled with nothing waiting renders no chip — the toggle already says "off"', () => {
  for (const v of ['quiet', 'nothing-survived', '']) {
    assert.equal(chipState(false, v, 0), null);
  }
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

// WP-182: the empty Matches page states the last run's verdict in one
// sentence — verdictMeta's vocabulary, plus a recovery pointer where
// the verdict names one.
test('verdictSentence: quiet states the count and outcome', () => {
  assert.deepEqual(
    verdictSentence({ verdict: 'quiet', postingsNew: 42 }),
    { text: 'Quiet — 42 postings, none fit.', dotClass: DOT_TONE.faint, href: null, pointer: '' },
  );
});

test('verdictSentence: nothing-survived counts the dismissals', () => {
  assert.deepEqual(
    verdictSentence({ verdict: 'nothing-survived', postingsNew: 17, postingsDismissed: 17 }),
    { text: 'Nothing survived — 17 postings, all dismissed.', dotClass: DOT_TONE.blue, href: null, pointer: '' },
  );
});

test('verdictSentence: waiting-on-you points at the review queue', () => {
  assert.deepEqual(
    verdictSentence({ verdict: 'waiting-on-you', postingsShortlisted: 3 }),
    { text: 'Needs you — 3 shortlists waiting in the review queue.', dotClass: DOT_TONE.amber, href: null, pointer: '' },
  );
});

test('verdictSentence: degraded points at the Settings run log', () => {
  assert.deepEqual(
    verdictSentence({ verdict: 'degraded', stageErrors: ['sweep ncbs: timeout'] }),
    { text: 'Degraded — sweep ncbs: timeout.', dotClass: DOT_TONE.red, href: '/settings', pointer: 'run log in Settings' },
  );
});

test('verdictSentence: no run is null — never a fabricated verdict', () => {
  assert.equal(verdictSentence(null), null);
  assert.equal(verdictSentence(undefined), null);
});

test('verdictSentence: no verdict yet (run in progress) is null', () => {
  assert.equal(verdictSentence({ verdict: '', startedAt: '2026-10-20T06:00:00' }), null);
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

// progressLine: the live narration from a running row's stage facts.
test('progressLine: per-stage fragments carry the real facts', () => {
  const running = (run) => ({ running: true, lastRun: run });
  assert.equal(progressLine(running({ stage: 'sweep', stageOf: '3/21', stageLabel: 'Instem' })),
    'fetching jobs from Instem — board 3 of 21');
  assert.equal(progressLine(running({ stage: 'read', stageOf: '5/12' })),
    'reading postings — 5 of 12');
  assert.equal(progressLine(running({ stage: 'curate', stageOf: '5/12' })),
    'scoring 5 of 12 against your profile');
  assert.equal(progressLine(running({ stage: 'discover' })),
    'scanning companies from your brief');
  assert.equal(progressLine(running({ stage: 'select' })),
    'picking sources worth sweeping');
});

test('progressLine: no stage facts yet is null, never an invented line', () => {
  assert.equal(progressLine({ running: true, lastRun: {} }), null);
  assert.equal(progressLine({ running: true }), null);
  assert.equal(progressLine(null), null);
});

test('progressLine: not running is null even with facts left over', () => {
  assert.equal(progressLine({ running: false, lastRun: { stage: 'sweep', stageOf: '1/2', stageLabel: 'X' } }), null);
});

test('progressLine: unknown stage id is null — the store carries facts, not sentences', () => {
  assert.equal(progressLine({ running: true, lastRun: { stage: 'future-stage', stageOf: '1/2' } }), null);
});
