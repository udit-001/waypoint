// Date formatting helpers shared across all views.
// Each variant is a named export so call sites are self-documenting.
// All return '' for empty/null input; callers handle their own fallback.
// Scraper dates are hostile input — new Date(garbage) does NOT throw
// (it yields an Invalid Date whose toLocale* renders "Invalid Date"),
// so the guard is isNaN on the timestamp: unparsable input is returned
// unchanged for the caller to fall back on.
function parseable(d) {
  const dt = new Date(d);
  return isNaN(dt.getTime()) ? null : dt;
}

export function formatDate(d) {
  if (!d) return '';
  const dt = parseable(d);
  if (!dt) return d;
  return dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export function formatDateShort(d) {
  if (!d) return '';
  const dt = parseable(d);
  if (!dt) return d;
  return dt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

export function formatDateTime(d) {
  if (!d) return '';
  const dt = parseable(d);
  if (!dt) return d;
  return dt.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

export function formatDateFull(d) {
  if (!d) return '';
  const dt = parseable(d);
  if (!dt) return d;
  return dt.toLocaleString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

// Partial-ISO month formatter (YYYY-MM → 'Mar 2023') for experience/education
// entry dates. Parsed manually (no Date object) so a timezone can never shift
// the displayed month. Invalid input is returned unchanged.
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

export function formatMonth(ym) {
  if (!ym) return '';
  const m = /^(\d{4})-(\d{2})$/.exec(ym);
  if (!m) return ym;
  const month = Number(m[2]);
  if (month < 1 || month > 12) return ym;
  return `${MONTHS[month - 1]} ${m[1]}`;
}
