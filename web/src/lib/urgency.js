// WP-95 — urgency flag for a job, used by the List row's urgency slot
// and the Kanban card's urgency line. Encodes the "what should I look
// at next" signal that survives the layout switch.
//
// Inputs: the job, plus today's date (injectable for tests).
// Output: one of
//   { kind: 'stale',    label: '22d stale',  tone: 'red'|'amber', days }
//   { kind: 'deadline', label: '5d left',    tone: 'red'|'amber', days }
//   { kind: 'closed',   label: 'Closed'|'Withdrawn', tone: 'muted' }
//   { kind: 'offer',    label: 'No deadline',         tone: 'muted' }
//   { kind: 'none',     label: '—',                    tone: 'muted' }
//
// Ordering: stale > deadline > closed > offer > none. A stale applied
// job with an upcoming deadline still shows as stale — silence about
// the deadline is intentional (the response is the bottleneck, not the
// date).

const STALE_THRESHOLD_DAYS = 14;
const STALE_DEEP_THRESHOLD_DAYS = 21;
const DEADLINE_SOON_THRESHOLD_DAYS = 7;

function daysSince(dateStr, now) {
  if (!dateStr) return null;
  const d = new Date(dateStr);
  if (isNaN(d)) return null;
  return Math.floor((now.getTime() - d.getTime()) / 86400000);
}

/** Local version of deadlineDaysLeft that accepts an explicit `now`
 *  (deadlineDaysLeft uses Date.now() and can't be injected). Pure
 *  ceiling math — same semantics as lib/deadline.js. */
function daysUntil(dateStr, now) {
  if (!dateStr) return null;
  const d = new Date(dateStr);
  if (isNaN(d)) return null;
  return Math.ceil((d.getTime() - now.getTime()) / 86400000);
}

export function urgencyFor(job, now = new Date()) {
  if (!job) return { kind: 'none', label: '—', tone: 'muted' };

  // Stale: applied with no response for > 14 days.
  if (job.status === 'Applied' && job.appliedDate) {
    const ds = daysSince(job.appliedDate, now);
    if (ds !== null && ds > STALE_THRESHOLD_DAYS) {
      return {
        kind: 'stale',
        days: ds,
        label: `${ds}d stale`,
        tone: ds > STALE_DEEP_THRESHOLD_DAYS ? 'red' : 'amber',
      };
    }
  }

  // Deadline bucket — only meaningful when one is set.
  if (job.date) {
    const dl = daysUntil(job.date, now);
    if (dl !== null) {
      if (dl < 0) {
        // Deadline already passed. Only call it "overdue" for Not Applied
        // — an applied job already discharged the deadline's purpose.
        if (job.status === 'Not Applied') {
          return {
            kind: 'deadline',
            days: dl,
            label: `${Math.abs(dl)}d overdue`,
            tone: 'red',
          };
        }
      } else if (dl <= DEADLINE_SOON_THRESHOLD_DAYS) {
        return {
          kind: 'deadline',
          days: dl,
          label: dl === 0 ? 'Today' : `${dl}d left`,
          tone: 'amber',
        };
      }
      // Deadline > 7d out — no urgency signal, fall through to the
      // status-based defaults below.
    }
  }

  if (job.status === 'Rejected')  return { kind: 'closed', label: 'Closed', tone: 'muted' };
  if (job.status === 'Withdrawn') return { kind: 'closed', label: 'Withdrawn', tone: 'muted' };
  if (job.status === 'Offer')     return { kind: 'offer', label: 'No deadline', tone: 'muted' };

  return { kind: 'none', label: '—', tone: 'muted' };
}
