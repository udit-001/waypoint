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

// briefStatus summarizes a brief's open-frontier state for the surface: the
// Preferences tab badge and the preferences card header both render from it.
// A complete brief is "ready"; an incomplete one shows how many preferences
// are still open. Defensive about a missing `open` list (the brief always
// has one in practice).
export function briefStatus(brief) {
  const open = brief?.open ?? [];
  const complete = Boolean(brief?.complete);
  const openCount = complete ? 0 : open.length;
  return {
    complete,
    openCount,
    label: complete
      ? 'All set — ready to search.'
      : `${openCount} preference${openCount === 1 ? '' : 's'} still to set.`,
  };
}
