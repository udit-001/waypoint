// Autopilot settings-card derivations (WP-183) — the pure logic behind
// the tabbed card, kept out of the component so node --test can pin it.
// Copy matches the locked Harbor variation 4 verbatim.

// chipState answers "does anything need me?" in one chip. Order is a
// decision, not an accident: people over machines — a queue waiting
// (amber) outranks a broken run (red); otherwise nothing needs you.
// The waiting test is the live queue depth, not the last verdict: a
// quiet run after shortlists landed still leaves people waiting.
export function chipState(verdict, pendingShortlists) {
  if (pendingShortlists > 0) {
    const noun = pendingShortlists === 1 ? 'posting' : 'postings';
    return { tone: 'amber', copy: `${pendingShortlists} ${noun} waiting` };
  }
  if (verdict === 'degraded') {
    return { tone: 'red', copy: 'degraded' };
  }
  return { tone: 'green', copy: 'nothing needs you' };
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
