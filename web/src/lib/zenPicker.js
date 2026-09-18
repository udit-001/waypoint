// Option-building for the zen model picker (WP-163): friendly names from
// the curated catalog, API-family badge suffixes, and a saved pick that is
// no longer in the catalog kept visible so it can't be silently lost.
// Pure logic — unit-tested; the Svelte component renders the result.

export const DefaultZenModel = 'big-pickle';

const familyLabel = {
  'openai-completions': 'chat',
  'openai-responses': 'responses',
  'anthropic-messages': 'anthropic',
};

// zenOptions returns the picker's option list: the shipped default first
// (blank value), then the catalog's entries with friendly names + family
// badges, then the saved selection when it's not in the catalog.
export function zenOptions(models = [], saved = '') {
  const options = [{ value: '', label: `Default (${DefaultZenModel})` }];
  for (const m of models || []) {
    if (!m?.id) continue;
    const badge = m.api ? (familyLabel[m.api] ?? m.api) : null;
    options.push({ value: m.id, label: (m.name || m.id) + (badge ? ` · ${badge}` : '') });
  }
  if (saved && !(models || []).some((x) => x?.id === saved)) {
    options.push({ value: saved, label: saved });
  }
  return options;
}
