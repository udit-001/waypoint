// applyFilter applies the shared 5-dimension filter to a job list.
// Used by every view that consumes the shared filter store (WP-96).
//
// Dimensions:
//   - categories    multi-value   job.category matches any
//   - statuses      multi-value   job.status matches any
//   - deadlineBucket single       see bucketFor(job) below
//   - stale         toggle        Applied AND appliedDate > 14d ago
//   - textQuery     string        case-insensitive substring over
//                                 position/company/category/notes/location
//
// Views with additional local filters (e.g. Kanban's column status) compose
// on top of the returned array.

import { deadlineDaysLeft } from './deadline.js';

const STALE_THRESHOLD_DAYS = 14;

/** Compute which deadline bucket a job falls into, or null if none.
 *  Buckets are mutually exclusive by definition — a job is in exactly
 *  one bucket (or none, if its deadline is >30d away and not overdue). */
function bucketFor(job) {
  if (!job.date) return 'no-date';
  const days = deadlineDaysLeft(job.date);
  if (days === null) return 'no-date'; // unparseable date
  if (days < 0) {
    // Deadline passed. Overdue only if status = Not Applied — if you
    // applied, the deadline served its purpose (not overdue even if late).
    if (job.status === 'Not Applied') return 'overdue';
    return null;
  }
  if (days <= 7) return 'this-week';
  if (days <= 30) return 'this-month';
  return null; // deadline > 30d away — no bucket match
}

/** Stale predicate: Applied with no response for >14 days.
 *  Uses Math.floor so the boundary is exact — "more than 14 full days"
 *  means 15+ days, not 14.5 (which Math.round would round up to 15). */
function isStale(job) {
  if (job.status !== 'Applied' || !job.appliedDate) return false;
  const days = Math.floor((Date.now() - new Date(job.appliedDate).getTime()) / 86400000);
  return days > STALE_THRESHOLD_DAYS;
}

/** Text-query predicate: case-insensitive substring over the job's
 *  text fields. Mirrors TableView's local search fields. */
function matchesText(job, q) {
  const needle = q.toLowerCase();
  if (!needle) return false;
  if (job.company && job.company.toLowerCase().includes(needle)) return true;
  if (job.position && job.position.toLowerCase().includes(needle)) return true;
  if (job.category && job.category.toLowerCase().includes(needle)) return true;
  if (job.notes && job.notes.toLowerCase().includes(needle)) return true;
  if (job.location && job.location.toLowerCase().includes(needle)) return true;
  return false;
}

export function applyFilter(jobs, filter) {
  let result = jobs || [];

  if (filter.categories && filter.categories.length) {
    // Treat missing category as '' so the "Uncategorized" pseudo-entry
    // (which filters on '') matches jobs with no category.
    result = result.filter(j => filter.categories.includes(j.category || ''));
  }
  if (filter.statuses && filter.statuses.length) {
    result = result.filter(j => filter.statuses.includes(j.status));
  }
  if (filter.deadlineBucket) {
    result = result.filter(j => bucketFor(j) === filter.deadlineBucket);
  }
  if (filter.stale) {
    result = result.filter(isStale);
  }
  const q = (filter.textQuery || '').trim();
  if (q) {
    result = result.filter(j => matchesText(j, q));
  }

  return result;
}

// Exported for tests + WP-97 triage strip.
export { bucketFor, isStale, matchesText, STALE_THRESHOLD_DAYS };
