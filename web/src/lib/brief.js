// Brief display helpers. The stored brief is normalized to a match form
// (case-folded, trimmed) so values match cleanly during search. The web
// renders a prettified sentence-case form instead. Prettification is purely
// cosmetic — it never feeds a search, and the stored value is never touched
// by it.

// prettify sentence-cases a single stored value, e.g. "golang" → "Golang".
export function prettify(value) {
  if (!value) return '';
  return value.charAt(0).toUpperCase() + value.slice(1);
}

// prettifyList maps prettify over an array of stored values.
export function prettifyList(values) {
  if (!values) return [];
  return values.map(prettify);
}
