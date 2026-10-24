// Autopilot settings-card derivations (WP-183) — the pure logic behind
// the tabbed card, kept out of the component so node --test can pin it.
// Copy matches the locked Harbor variation 4 verbatim.

// chipState answers "does anything need me?" in one chip. Order is a
// decision, not an accident: people over machines — a queue waiting
// (amber) outranks a broken run (red); otherwise nothing needs you.
// The waiting test is the live queue depth, not the last verdict: a
// quiet run after shortlists landed still leaves people waiting.
//
// The green idle tone carries meaning only while autopilot is running:
// next to an off toggle it restates what the toggle already says
// ("Currently off"), so disabled with nothing waiting renders no chip —
// the caller distinguishes "no signal" (null) from a real state. A
// waiting or degraded state still shows when disabled: shortlists land
// while autopilot is off, and a broken run stays broken until someone
// looks.
export function chipState(enabled, verdict, pendingShortlists) {
  if (pendingShortlists > 0) {
    const noun = pendingShortlists === 1 ? 'posting' : 'postings';
    return { tone: 'amber', copy: `${pendingShortlists} ${noun} waiting` };
  }
  if (verdict === 'degraded') {
    return { tone: 'red', copy: 'degraded' };
  }
  if (!enabled) {
    return null;
  }
  return { tone: 'green', copy: 'nothing needs you' };
}

// scheduleStatus derives the autopilot progress state from the payload
// GET /api/autopilot returns (the payload itself is computed from the same
// scheduling seam the daemon polls — the web reports, never guesses).
// Returns { kind, at } or null when there is nothing to say (disabled or
// unknown state). Kinds:
//   'run-in-progress' — a cycle is mid-flight right now
//   'first-run'       — enabled, no finished cycle yet; the first cycle
//                       starts within PollInterval (30s) of enabling
//   'cadence-wait'    — steady state; at = the next cycle's Date
export function scheduleStatus(payload, now = new Date()) {
  if (!payload || !payload.enabled) return null;
  if (payload.running) return { kind: 'run-in-progress' };
  if (payload.firstRun) return { kind: 'first-run' };
  if (payload.nextRunAt) {
    const t = new Date(payload.nextRunAt);
    if (!Number.isNaN(t.getTime())) {
      // A past-or-now nextRunAt means the scheduler is already due —
      // that's first-run urgency, never a stale clock.
      if (t <= now) return { kind: 'first-run' };
      return { kind: 'cadence-wait', at: t };
    }
  }
  return null;
}

// clockTime renders a schedule Date as the brief "around HH:MM" form the
// surfaces quote. Empty/null-safe.
export function clockTime(d) {
  if (!d) return '';
  try {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
  } catch {
    return '';
  }
}

// verdictMeta renders one run as the artifact's verdict line:
// [chip] [one fragment naming the outcome].
export function verdictMeta(run) {
  const r = run || {};
  switch (r.verdict) {
    case 'quiet':
      return { chipClass: '', label: 'Quiet', what: `${r.postingsNew ?? 0} postings, none fit` };
    case 'nothing-survived':
      return { chipClass: 'blue', label: 'Nothing survived', what: `${r.postingsNew ?? 0} postings, all dismissed` };
    case 'waiting-on-you':
      return { chipClass: 'amber', label: 'Needs you', what: `${r.postingsShortlisted ?? 0} shortlists waiting` };
    case 'degraded': {
      const first = Array.isArray(r.stageErrors) && r.stageErrors.length > 0 ? r.stageErrors[0] : '';
      return { chipClass: 'red', label: 'Degraded', what: first || 'a stage errored — coverage partial' };
    }
    default:
      return { chipClass: '', label: 'Running', what: 'this page updates when it finishes' };
  }
}

// verdictSentence states the last run's verdict in one sentence for the
// empty Matches page (WP-182). Derived from verdictMeta's vocabulary —
// label + evidence fragment — plus a recovery pointer where the verdict
// names one: waiting-on-you -> the review queue (woven into the
// sentence), degraded -> the run log in Settings (the href/pointer
// pair). Returns null when there is nothing to state — no run at all,
// or a run still in progress (no verdict yet; the header status chip
// carries "running now") — so the never-run state keeps its own line
// and no fabricated verdict ever renders.
export function verdictSentence(run) {
  if (!run) return null;
  const m = verdictMeta(run);
  const dotClass = DOT_TONE[dotColor(run.verdict)];
  switch (run.verdict) {
    case 'waiting-on-you':
      return { text: `${m.label} — ${m.what} in the review queue.`, dotClass, href: null, pointer: '' };
    case 'degraded':
      return { text: `${m.label} — ${m.what}.`, dotClass, href: '/settings', pointer: 'run log in Settings' };
    case 'quiet':
    case 'nothing-survived':
      return { text: `${m.label} — ${m.what}.`, dotClass, href: null, pointer: '' };
    default:
      return null;
  }
}

// dotColor maps a verdict to its state hue name (color = state only):
// quiet idle, waiting amber, nothing-survived blue, degraded red.
export function dotColor(verdict) {
  switch (verdict) {
    case 'waiting-on-you': return 'amber';
    case 'nothing-survived': return 'blue';
    case 'degraded': return 'red';
    default: return 'faint';
  }
}

// Tone → class maps. They live here, next to the code that mints the
// tones — the component only renders what this module names.
export const CHIP_TONE = {
  slate: 'bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300',
  blue: 'bg-blue-50 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400',
  green: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-400',
  amber: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400',
  red: 'bg-red-50 text-red-700 dark:bg-red-900/40 dark:text-red-400',
};

export const DOT_TONE = {
  faint: 'bg-slate-300 dark:bg-slate-500',
  blue: 'bg-blue-400 dark:bg-blue-500',
  amber: 'bg-amber-500',
  red: 'bg-red-500',
};

// shortWhen renders a run timestamp in the artifact's three forms:
// "Today 06:12", "Yesterday", or the weekday. Local time — a run at
// 06:12 UTC must not read 11:42 to an IST user.
export function shortWhen(iso, now = new Date()) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const day = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diffDays = Math.round((day(now) - day(d)) / 86400000);
  if (diffDays <= 0) {
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    return `Today ${hh}:${mm}`;
  }
  if (diffDays === 1) return 'Yesterday';
  return d.toLocaleDateString('en-US', { weekday: 'short' });
}
