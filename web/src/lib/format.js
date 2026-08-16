// Date formatting helpers shared across all views.
// Each variant is a named export so call sites are self-documenting.
// All return '' for empty/null input; callers handle their own fallback.

export function formatDate(d) {
  if (!d) return '';
  try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }); }
  catch { return d; }
}

export function formatDateShort(d) {
  if (!d) return '';
  try { return new Date(d).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }); }
  catch { return d; }
}

export function formatDateTime(d) {
  if (!d) return '';
  try { return new Date(d).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }); }
  catch { return d; }
}

export function formatDateFull(d) {
  if (!d) return '';
  try { return new Date(d).toLocaleString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' }); }
  catch { return d; }
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
