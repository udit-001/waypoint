/**
 * LinkedIn import composable — the fetch→preview→apply flow.
 * Shared by Profile.svelte and GetStarted.svelte (two adapters,
 * one seam). Accepts the api module as a dependency so callers
 * and tests cross the same seam.
 */

export function createLinkedInImport(api) {
  let url = $state('');
  let fetching = $state(false);
  let error = $state(null);
  let preview = $state(null);
  let applied = $state(false);
  let appliedSummary = $state(null);
  let applyTimer = null;

  function summaryText(sum) {
    const n = (a) => a?.length || 0;
    const parts = [];
    if (n(sum.experienceAdded)) parts.push(`${n(sum.experienceAdded)} role${n(sum.experienceAdded) === 1 ? '' : 's'} added`);
    if (n(sum.experienceUpdated)) parts.push(`${n(sum.experienceUpdated)} role${n(sum.experienceUpdated) === 1 ? '' : 's'} updated`);
    if (n(sum.educationAdded)) parts.push(`${n(sum.educationAdded)} education added`);
    if (n(sum.educationUpdated)) parts.push(`${n(sum.educationUpdated)} education updated`);
    if (n(sum.skillsAdded)) parts.push(`+${n(sum.skillsAdded)} skills`);
    return parts.length ? parts.join(' · ') : 'no changes';
  }

  async function fetchProfile() {
    if (!url.trim() || fetching) return;
    fetching = true;
    error = null;
    try {
      preview = await api.importLinkedInProfile(url);
    } catch (e) {
      error = e.message || 'Could not fetch profile';
    }
    fetching = false;
  }

  async function apply() {
    if (!preview || fetching) return;
    fetching = true;
    error = null;
    try {
      await api.updateBrief(preview.doc);
      await api.profile.refresh();
      appliedSummary = summaryText(preview.summary);
      preview = null;
      url = '';
      applied = true;
      clearTimeout(applyTimer);
      applyTimer = setTimeout(() => { applied = false; appliedSummary = null; }, 4000);
    } catch (e) {
      error = e.message || 'Apply failed';
    }
    fetching = false;
  }

  function discard() {
    preview = null;
    error = null;
  }

  return {
    get url() { return url; },
    set url(v) { url = v; },
    get fetching() { return fetching; },
    get error() { return error; },
    get preview() { return preview; },
    get applied() { return applied; },
    get appliedSummary() { return appliedSummary; },
    fetchProfile,
    apply,
    discard,
  };
}
