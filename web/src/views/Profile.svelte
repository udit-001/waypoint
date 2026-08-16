<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import Field from '../components/Field.svelte';
  import ChipInput from '../components/ChipInput.svelte';
  import TextInput from '../components/TextInput.svelte';
  import SelectInput from '../components/SelectInput.svelte';
  import EntryEditor from '../components/EntryEditor.svelte';
  import { prettify, briefStatus } from '../lib/brief.js';
  import * as api from '../stores/api.svelte.js';
  import { getPage, setEditing } from '../stores/page.svelte.js';
  import { getProfileTabs } from '../stores/profileTabs.svelte.js';

  const page = getPage();

  // Two tabs on one route (/profile): the identity cards vs the job-search
  // brief. The active tab persists to ?tab= + localStorage (lib/profileTabs).
  const tabs = getProfileTabs();
  const prefsStatus = $derived(briefStatus(briefData));

  // View-mode helpers (WP-117): clean read-only render. Empty fields hide;
  // each empty section shows one quiet line.
  const PILL_CLS = 'inline-flex items-center bg-slate-100 dark:bg-slate-700 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-200 rounded-full px-2.5 py-0.5 text-xs';
  function optionLabel(value, options) {
    return options.find((o) => o.value === value)?.label || '';
  }
  function floorLine(f) {
    return f.currency ? `${f.currency} ${f.amount} (${f.region})` : `${f.amount} (${f.region})`;
  }
  const PERSONAL_FIELDS = [
    { label: 'Full Name', key: 'name' },
    { label: 'Professional Title', key: 'title' },
    { label: 'Email', key: 'email' },
    { label: 'Phone', key: 'phone' },
    { label: 'Industry', key: 'industry' },
    { label: 'Current Location', key: 'currentLocation' },
  ];
  function fieldSet(f) {
    return Boolean((profileData?.[f.key] || '').trim());
  }

  let profileData = $state(null);
  let briefData = $state(null);

  let saving = $state(false);
  let saveError = $state(null);
  let saveOk = $state(false);
  let saveOkTimer = null;

  // Editable salary-floor rows (free-text inputs commit on blur).
  let salaryRows = $state([]);

  // isEmpty drives the import card: an empty profile gets the full seed card
  // (fetch is the first step); a populated one gets a collapsed "Update from
  // LinkedIn" affordance, so the least-frequent action never dominates.
  let isEmpty = $derived(
    !(profileData?.name?.trim()) &&
    !(profileData?.title?.trim()) &&
    !(profileData?.skills?.length) &&
    !(profileData?.experience?.length) &&
    !(profileData?.education?.length),
  );

  // Skills wall: an imported profile can carry dozens of noisy pills. Show
  // the first 10 in read-only view with a +N more expand, so the card stays
  // scannable (surface discipline: prominence reflects importance).
  const SKILLS_CAP = 10;
  let skillsExpanded = $state(false);
  let visibleSkills = $derived(skillsExpanded ? (profileData?.skills ?? []) : (profileData?.skills ?? []).slice(0, SKILLS_CAP));

  // LinkedIn import (Exa MCP): fetch → preview → apply via the existing
  // PATCH route. The import endpoint merges and never writes; Apply is the
  // only writer. The preview is a {doc, summary} diff — added/updated/kept.
  let linkedinUrl = $state('');
  let importing = $state(false);
  let importError = $state(null);
  let importPreview = $state(null);
  let importActive = $state(false);
  let importApplied = $state(false);
  let importAppliedSummary = $state(null);
  let importAppliedTimer = null;

  async function fetchLinkedIn() {
    importing = true;
    importError = null;
    importApplied = false;
    importAppliedSummary = null;
    try {
      importPreview = await api.importLinkedInProfile(linkedinUrl);
      importActive = false;
    } catch (e) {
      importError = e.message || 'Could not fetch profile';
    } finally {
      importing = false;
    }
  }

  function discardImport() {
    importPreview = null;
    importError = null;
    importActive = false;
  }

  // One-line summary of what a merge changed, for post-apply feedback.
  function importSummaryText(sum) {
    const n = (a) => a?.length || 0;
    const parts = [];
    if (n(sum.experienceAdded)) parts.push(`${n(sum.experienceAdded)} role${n(sum.experienceAdded) === 1 ? '' : 's'} added`);
    if (n(sum.experienceUpdated)) parts.push(`${n(sum.experienceUpdated)} role${n(sum.experienceUpdated) === 1 ? '' : 's'} updated`);
    if (n(sum.educationAdded)) parts.push(`${n(sum.educationAdded)} education added`);
    if (n(sum.educationUpdated)) parts.push(`${n(sum.educationUpdated)} education updated`);
    if (n(sum.skillsAdded)) parts.push(`+${n(sum.skillsAdded)} skills`);
    return parts.length ? parts.join(' · ') : 'no changes';
  }

  // Mirrors save() but surfaces failures inside the import card (save()
  // swallows errors into a header-only indicator).
  async function applyLinkedIn() {
    saving = true;
    importError = null;
    importApplied = false;
    importAppliedSummary = null;
    try {
      briefData = await api.updateBrief(importPreview.doc);
      await api.profile.refresh();
      profileData = api.profile.value;
      importAppliedSummary = importSummaryText(importPreview.summary);
      discardImport();
      linkedinUrl = '';
      importApplied = true;
      clearTimeout(importAppliedTimer);
      importAppliedTimer = setTimeout(() => {
        importApplied = false;
        importAppliedSummary = null;
      }, 4000);
    } catch (e) {
      importError = e.message || 'Apply failed';
    } finally {
      saving = false;
    }
    syncSalaryRows();
  }

  const REMOTE_OPTIONS = [
    { value: '', label: 'Any' },
    { value: 'remote', label: 'Remote' },
    { value: 'hybrid', label: 'Hybrid' },
    { value: 'onsite', label: 'Onsite' },
  ];
  const VISA_OPTIONS = [
    { value: '', label: 'Any' },
    { value: 'yes', label: 'Yes' },
    { value: 'no', label: 'No' },
  ];
  const SENIORITY_OPTIONS = [
    { value: '', label: 'Any' },
    { value: 'junior', label: 'Junior' },
    { value: 'mid', label: 'Mid' },
    { value: 'senior', label: 'Senior' },
  ];

  // Profile fields read back via GET /api/profile; touching one of these
  // refreshes profileData after the write (brief-only edits don't need to).
  const PROFILE_KEYS = new Set([
    'name', 'email', 'phone', 'title', 'industry',
    'currentLocation', 'skills', 'experience', 'education', 'seniority',
  ]);

  onMount(async () => {
    setPage({ title: 'Profile' });

    const onEsc = (e) => {
      if (e.key === 'Escape' && page.editing) setEditing(false);
    };
    window.addEventListener('keydown', onEsc);

    await Promise.all([api.profile.ensure(), api.brief.ensure()]);
    profileData = api.profile.value;
    briefData = api.brief.value;
    syncSalaryRows();

    // First-run (WP-117): an empty profile opens edit mode by itself; the
    // clean view resets on every visit, so edit mode never sticks.
    setEditing(isEmpty);

    return () => window.removeEventListener('keydown', onEsc);
  });

  async function save(fields) {
    saving = true;
    saveError = null;
    saveOk = false;
    try {
      briefData = await api.updateBrief(fields);
      if (Object.keys(fields).some((k) => PROFILE_KEYS.has(k))) {
        await api.profile.refresh();
        profileData = api.profile.value;
      }
      saveOk = true;
      clearTimeout(saveOkTimer);
      saveOkTimer = setTimeout(() => (saveOk = false), 1800);
    } catch (e) {
      saveError = e.message || 'Save failed';
    } finally {
      saving = false;
    }
    syncSalaryRows();
  }

  function syncSalaryRows() {
    const floors = briefData?.constraints?.salary_floor || [];
    salaryRows = floors.map((f) => ({ region: f.region || '', amount: String(f.amount ?? '') }));
  }

  function setVisa(value) {
    if (briefData) briefData.constraints.visa_sponsorship = value;
    save({ visaSponsorship: value });
  }

  function setRemote(value) {
    if (briefData) briefData.preferences.remote = value;
    save({ remote: value });
  }

  // The brief's preference keys are the profile doc keys in snake_case — the
  // bridge keeps the two documents' vocabularies separate.
  const BRIEF_PREF_KEYS = {
    locationPreference: 'location_preference',
    companies: 'companies',
    avoidCompanies: 'avoid_companies',
    keywords: 'keywords',
    dealbreakers: 'dealbreakers',
  };

  function setList(key, value) {
    const briefKey = BRIEF_PREF_KEYS[key];
    if (briefData && briefKey) briefData.preferences[briefKey] = value;
    save({ [key]: value });
  }

  function addSalaryRow() {
    salaryRows = [...salaryRows, { region: '', amount: '' }];
  }

  function removeSalaryRow(i) {
    salaryRows = salaryRows.filter((_, idx) => idx !== i);
    commitSalaryRows();
  }

  function commitSalaryRows() {
    const floors = salaryRows
      .map((r) => ({ region: r.region.trim(), amount: Number(r.amount) }))
      .filter((f) => f.region !== '' && Number.isFinite(f.amount) && f.amount > 0);
    save({ salaryFloor: floors });
  }
</script>

<div class="space-y-4">
  <!-- Tabs: profile (identity) vs job-search preferences (brief config).
       The active tab persists via ?tab= + localStorage. Pill styling matches
       the Applications view's List/Kanban toggle (TopBar). -->
  <div class="flex items-center gap-0.5 p-0.5 rounded-md bg-slate-100 dark:bg-slate-700 shadow-[inset_0_1px_2px_rgba(0,0,0,0.10)] w-fit">
    <button
      type="button"
      class="px-3 py-1.5 rounded-md text-sm font-medium flex items-center gap-1.5 transition-colors cursor-pointer {tabs.current === 'profile' ? 'bg-white dark:bg-slate-600 text-slate-800 dark:text-slate-100 shadow-sm' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
      onclick={() => tabs.set('profile')}
    >{@html iconSvg('user', 15)} Profile</button>
    <button
      type="button"
      class="px-3 py-1.5 rounded-md text-sm font-medium flex items-center gap-1.5 transition-colors cursor-pointer {tabs.current === 'preferences' ? 'bg-white dark:bg-slate-600 text-slate-800 dark:text-slate-100 shadow-sm' : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
      onclick={() => tabs.set('preferences')}
    >{@html iconSvg('target', 15)} Job Search Preferences
      {#if briefData && !prefsStatus.complete}
        <span class="rounded-full px-1.5 text-[10px] font-semibold tabular-nums bg-slate-700 dark:bg-slate-900 text-white">{prefsStatus.openCount}</span>
      {/if}
    </button>
  </div>

  {#if tabs.current === 'profile'}
  <!-- Import from LinkedIn (Exa MCP): the full card shows when the profile is
       empty (seed) or a fetch/preview flow is active; when populated and
       idle, one quiet "Update from LinkedIn" button lives in the Personal
       Info header. The endpoint merges (never deletes) and never writes —
       Apply PATCHes the merged doc through the existing route. -->
  {#if isEmpty || importActive || importing || importError || importPreview || importApplied}
  <Card hover={false}>
    {#snippet fetchRow()}
      <div class="mt-4 flex items-center gap-2">
        <input
          class="wp-input flex-1 min-w-0 placeholder:text-slate-400"
          placeholder="https://www.linkedin.com/in/username"
          aria-label="LinkedIn profile URL"
          bind:value={linkedinUrl}
          onkeydown={(e) => { if (e.key === 'Enter' && !importing && linkedinUrl.trim()) fetchLinkedIn(); }}
        />
        <button
          type="button"
          class="inline-flex items-center gap-1.5 h-[38px] px-3 rounded-lg text-sm font-medium bg-slate-700 dark:bg-slate-900 text-white transition-colors cursor-pointer disabled:opacity-50 shrink-0"
          disabled={importing || !linkedinUrl.trim()}
          onclick={fetchLinkedIn}
        >{@html iconSvg('linkedin', 15)} {importing ? 'Fetching…' : 'Fetch profile'}</button>
      </div>
    {/snippet}

    <div class="flex items-start justify-between">
      <div>
        <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200">
          {@html iconSvg('linkedin', 18)} {isEmpty ? 'Import from LinkedIn' : 'Update from LinkedIn'}
        </h3>
        <p class="text-xs text-slate-400 mt-2">
          {isEmpty
            ? 'Fetch a public profile and prefill name, title, location, skills, experience, and education.'
            : 'Sync roles, dates, skills, and education from your public LinkedIn profile.'}
        </p>
      </div>
      {#if importApplied}
        <span class="text-xs text-emerald-600 dark:text-emerald-400 shrink-0 text-right">
          {importAppliedSummary ? `Updated ✓ ${importAppliedSummary}` : 'Applied ✓'}
        </span>
      {/if}
    </div>

    {#if importPreview}
      <!-- Diff preview: exactly what Apply will write. -->
      <div class="mt-4 space-y-4">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
          {#if importPreview.doc.name}
            <div><label class="wp-label">Name</label><div class="text-sm text-slate-700 dark:text-slate-200 break-words">{importPreview.doc.name}</div></div>
          {/if}
          {#if importPreview.doc.title}
            <div><label class="wp-label">Professional Title</label><div class="text-sm text-slate-700 dark:text-slate-200 break-words">{importPreview.doc.title}</div></div>
          {/if}
          {#if importPreview.doc.currentLocation}
            <div><label class="wp-label">Current Location</label><div class="text-sm text-slate-700 dark:text-slate-200 break-words">{importPreview.doc.currentLocation}</div></div>
          {/if}
        </div>
        {#if (importPreview.summary.skillsAdded ?? []).length}
          <div>
            <label class="wp-label">{isEmpty ? 'Skills' : 'New skills'}</label>
            <div class="flex flex-wrap gap-1.5">
              {#each importPreview.summary.skillsAdded as s}
                <span class={PILL_CLS}>{s}</span>
              {/each}
            </div>
          </div>
        {/if}
        {#if (importPreview.summary.experienceAdded ?? []).length}
          <div>
            <label class="wp-label">{isEmpty ? 'Experience' : 'New experience'}</label>
            <EntryEditor entries={importPreview.summary.experienceAdded} primaryKey="title" secondaryKey="company" readonly />
          </div>
        {/if}
        {#if (importPreview.summary.experienceUpdated ?? []).length}
          <div>
            <label class="wp-label">Updated experience</label>
            <EntryEditor entries={importPreview.summary.experienceUpdated} primaryKey="title" secondaryKey="company" readonly />
          </div>
        {/if}
        {#if importPreview.summary.experienceKept > 0}
          <p class="text-xs text-slate-400">{importPreview.summary.experienceKept} existing role{importPreview.summary.experienceKept === 1 ? '' : 's'} unchanged.</p>
        {/if}
        {#if (importPreview.summary.educationAdded ?? []).length}
          <div>
            <label class="wp-label">{isEmpty ? 'Education' : 'New education'}</label>
            <EntryEditor entries={importPreview.summary.educationAdded} primaryKey="degree" secondaryKey="institution" readonly />
          </div>
        {/if}
        {#if (importPreview.summary.educationUpdated ?? []).length}
          <div>
            <label class="wp-label">Updated education</label>
            <EntryEditor entries={importPreview.summary.educationUpdated} primaryKey="degree" secondaryKey="institution" readonly />
          </div>
        {/if}
        {#if importPreview.summary.educationKept > 0}
          <p class="text-xs text-slate-400">{importPreview.summary.educationKept} existing education entr{importPreview.summary.educationKept === 1 ? 'y' : 'ies'} unchanged.</p>
        {/if}
        <div class="flex items-center gap-3">
          <button
            type="button"
            class="inline-flex items-center gap-1.5 h-[38px] px-3 rounded-lg text-sm font-medium bg-slate-700 dark:bg-slate-900 text-white transition-colors cursor-pointer disabled:opacity-50"
            disabled={saving}
            onclick={applyLinkedIn}
          >{@html iconSvg('check', 15)} Apply to profile</button>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-sm text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
            onclick={discardImport}
          >Discard</button>
          {#if saving}<span class="text-xs text-slate-400">Saving…</span>{/if}
        </div>
        <p class="text-xs text-slate-400">
          {isEmpty
            ? 'Applying replaces current name, title, location, skills, experience, and education.'
            : 'Adds new roles and skills, updates matched ones, and keeps everything else. Nothing is removed.'}
        </p>
      </div>
    {:else if isEmpty || importActive}
      <!-- Seed / update, expanded: the fetch row is the step. -->
      {@render fetchRow()}
      {#if importing}
        <p class="text-xs text-slate-400 mt-2">Fetching profile — this usually takes 5–15 seconds.</p>
      {:else if isEmpty}
        <p class="text-xs text-slate-400 mt-2">The profile must be public — private profiles won't load.</p>
      {/if}
    {/if}
    {#if importError}
      <p class="text-xs text-red-600 dark:text-red-400 mt-2">{importError}</p>
    {/if}
  </Card>
  {/if}

  {#if profileData}
    <!-- Personal Info -->
    <Card hover={false}>
      <div class="flex items-center justify-between gap-3 mb-4">
        <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200">
          {@html iconSvg("user", 18)} Personal Info
        </h3>
        {#if !isEmpty && !importActive && !importing && !importError && !importPreview && !importApplied}
          <!-- Update, collapsed: one quiet affordance in the header. -->
          <button
            type="button"
            class="inline-flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0 shrink-0"
            onclick={() => (importActive = true)}
          >{@html iconSvg('linkedin', 14)} Update from LinkedIn</button>
        {/if}
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
        {#if page.editing}
        <TextInput label="Full Name" value={profileData.name} placeholder="Jane Doe" oncommit={(v) => save({ name: v })} />
        <TextInput label="Professional Title" value={profileData.title} placeholder="Senior Engineer" oncommit={(v) => save({ title: v })} />
        <TextInput label="Email" type="email" value={profileData.email} placeholder="jane@example.com" oncommit={(v) => save({ email: v })} />
        <TextInput label="Phone" value={profileData.phone} placeholder="+1-555-0123" oncommit={(v) => save({ phone: v })} />
        <TextInput label="Industry" value={profileData.industry} placeholder="Biotech" oncommit={(v) => save({ industry: v })} />
        <TextInput label="Current Location" value={profileData.currentLocation} placeholder="Bengaluru" oncommit={(v) => save({ currentLocation: v })} />
        {#if briefData?.facts?.seniority}
          <div>
            <label class="wp-label">Seniority</label>
            <div class="text-sm text-slate-700 dark:text-slate-200 capitalize">{briefData.facts.seniority}</div>
          </div>
        {:else}
          <SelectInput
            label="Seniority"
            value={profileData.seniority || ''}
            options={SENIORITY_OPTIONS}
            onchange={(v) => save({ seniority: v })}
          />
        {/if}
        {:else}
        {#each PERSONAL_FIELDS as f}
          {#if fieldSet(f)}
            <div>
              <label class="wp-label">{f.label}</label>
              <div class="text-sm text-slate-700 dark:text-slate-200 break-words">{profileData[f.key]}</div>
            </div>
          {/if}
        {/each}
        {#if briefData?.facts?.seniority || (profileData.seniority || '').trim()}
          <div>
            <label class="wp-label">Seniority</label>
            <div class="text-sm text-slate-700 dark:text-slate-200 capitalize">{briefData?.facts?.seniority || profileData.seniority}</div>
          </div>
        {/if}
        {#if !PERSONAL_FIELDS.some(fieldSet) && !(briefData?.facts?.seniority || (profileData.seniority || '').trim())}
          <p class="text-sm text-slate-400 dark:text-slate-500">No personal info yet.</p>
        {/if}
        {/if}
      </div>
    </Card>

    <!-- Skills -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg('zap', 18)} Skills
      </h3>
      {#if page.editing}
        <ChipInput value={profileData.skills} placeholder="e.g. Go, React, AWS" onchange={(v) => save({ skills: v })} />
      {:else}
        {#if (profileData.skills ?? []).length}
          <div class="flex flex-wrap gap-1.5">
            {#each visibleSkills as s}
              <span class={PILL_CLS}>{s}</span>
            {/each}
          </div>
          {#if !skillsExpanded && (profileData.skills ?? []).length > SKILLS_CAP}
            <button
              type="button"
              class="mt-2.5 inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
              onclick={() => (skillsExpanded = true)}
            >+{(profileData.skills ?? []).length - SKILLS_CAP} more</button>
          {/if}
        {:else}
          <p class="text-sm text-slate-400 dark:text-slate-500">No skills yet.</p>
        {/if}
      {/if}
    </Card>

    <!-- Experience -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg('briefcase', 18)} Experience
      </h3>
      {#if page.editing}
        <EntryEditor
          entries={profileData.experience}
          primaryKey="title"
          primaryLabel="Title"
          primaryPlaceholder="Senior Software Engineer"
          secondaryKey="company"
          secondaryLabel="Company"
          secondaryPlaceholder="Acme Corp"
          descriptionPlaceholder="Key achievements, scope, tech used…"
          onchange={(v) => save({ experience: v })}
        />
      {:else}
        {#if (profileData.experience ?? []).length}
          <EntryEditor entries={profileData.experience} primaryKey="title" secondaryKey="company" readonly />
        {:else}
          <p class="text-sm text-slate-400 dark:text-slate-500">No experience yet.</p>
        {/if}
      {/if}
    </Card>

    <!-- Education -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg('grad', 18)} Education
      </h3>
      {#if page.editing}
        <EntryEditor
          entries={profileData.education}
          primaryKey="institution"
          primaryLabel="Institution"
          primaryPlaceholder="MIT"
          secondaryKey="degree"
          secondaryLabel="Degree"
          secondaryPlaceholder="BS Computer Science"
          descriptionPlaceholder="Focus areas, GPA, thesis…"
          onchange={(v) => save({ education: v })}
        />
      {:else}
        {#if (profileData.education ?? []).length}
          <EntryEditor entries={profileData.education} primaryKey="degree" secondaryKey="institution" readonly />
        {:else}
          <p class="text-sm text-slate-400 dark:text-slate-500">No education yet.</p>
        {/if}
      {/if}
    </Card>
  {:else if api.profile.loading}
    <Spinner text="Loading profile..." />
  {:else}
    <p class="text-sm text-slate-400">Profile not loaded (server may be down).</p>
  {/if}
  {:else}
  <!-- Job Search Preferences — the brief drives the agent's search; it is
       configuration, so it lives on its own tab. -->
  {#if briefData}
    <Card hover={false}>
      {#snippet pills(label, items)}
        {#if (items ?? []).length}
          <div>
            <label class="wp-label">{label}</label>
            <div class="flex flex-wrap gap-1.5">
              {#each items as v}
                <span class={PILL_CLS}>{prettify(v)}</span>
              {/each}
            </div>
          </div>
        {/if}
      {/snippet}

      <div class="flex items-start justify-between mb-6">
        <div>
          <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200">
            {@html iconSvg('target', 18)} Job Search Preferences
          </h3>
          <p class="text-xs text-slate-400 mt-2">{prefsStatus.label}</p>
        </div>
        <div class="flex items-center gap-2 text-xs shrink-0">
          {#if page.editing}
            {#if saving}
              <span class="text-slate-400">Saving…</span>
            {:else if saveOk}
              <span class="text-emerald-600 dark:text-emerald-400">Saved ✓</span>
            {/if}
            {#if saveError}
              <span class="text-red-600 dark:text-red-400">{saveError}</span>
            {/if}
          {/if}
        </div>
      </div>

      <!-- Preferences — primary, chip-editable -->
      <section>
        <h4 class="wp-section-title">Preferences</h4>
        {#if page.editing}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
          <SelectInput
            label="Remote"
            value={briefData.preferences.remote}
            options={REMOTE_OPTIONS}
            onchange={setRemote}
          />
          <Field label="Location Preference">
            <ChipInput
              value={briefData.preferences.location_preference}
              placeholder="e.g. Bengaluru, Remote"
              {prettify}
              onchange={(v) => setList('locationPreference', v)}
            />
          </Field>
          <Field label="Companies">
            <ChipInput
              value={briefData.preferences.companies}
              placeholder="e.g. Acme, Globex"
              {prettify}
              onchange={(v) => setList('companies', v)}
            />
          </Field>
          <Field label="Avoid Companies">
            <ChipInput
              value={briefData.preferences.avoid_companies}
              placeholder="e.g. Enron"
              {prettify}
              onchange={(v) => setList('avoidCompanies', v)}
            />
          </Field>
          <Field label="Keywords">
            <ChipInput
              value={briefData.preferences.keywords}
              placeholder="e.g. Go, Distributed systems"
              {prettify}
              onchange={(v) => setList('keywords', v)}
            />
          </Field>
          <Field label="Dealbreakers">
            <ChipInput
              value={briefData.preferences.dealbreakers}
              placeholder="e.g. Night shifts, Travel"
              {prettify}
              onchange={(v) => setList('dealbreakers', v)}
            />
          </Field>
        </div>
        {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
          {#if briefData.preferences.remote}
            <div>
              <label class="wp-label">Remote</label>
              <div class="text-sm text-slate-700 dark:text-slate-200">{optionLabel(briefData.preferences.remote, REMOTE_OPTIONS)}</div>
            </div>
          {/if}
          {@render pills('Location Preference', briefData.preferences.location_preference)}
          {@render pills('Companies', briefData.preferences.companies)}
          {@render pills('Avoid Companies', briefData.preferences.avoid_companies)}
          {@render pills('Keywords', briefData.preferences.keywords)}
          {@render pills('Dealbreakers', briefData.preferences.dealbreakers)}
        </div>
        {#if !briefData.preferences.remote && !(briefData.preferences.location_preference ?? []).length && !(briefData.preferences.companies ?? []).length && !(briefData.preferences.avoid_companies ?? []).length && !(briefData.preferences.keywords ?? []).length && !(briefData.preferences.dealbreakers ?? []).length}
          <p class="text-sm text-slate-400 dark:text-slate-500">No preferences set yet.</p>
        {/if}
        {/if}
      </section>

      <div class="my-6 border-t border-slate-100 dark:border-slate-700" />

      <!-- Constraints — editable -->
      <section>
        <h4 class="wp-section-title">Constraints</h4>
        {#if page.editing}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
          <SelectInput
            label="Visa Sponsorship"
            value={briefData.constraints.visa_sponsorship}
            options={VISA_OPTIONS}
            onchange={setVisa}
          />
          <Field label="Salary Floor">
            <div class="space-y-2">
              {#each salaryRows as row, i}
                <div class="flex items-center gap-1.5">
                  <input
                    class="wp-input flex-1 min-w-0 placeholder:text-slate-400"
                    placeholder="Region (e.g. IN)"
                    bind:value={salaryRows[i].region}
                    onblur={commitSalaryRows}
                  />
                  <input
                    class="wp-input flex-1 min-w-0 placeholder:text-slate-400"
                    placeholder="Amount"
                    bind:value={salaryRows[i].amount}
                    onblur={commitSalaryRows}
                  />
                  <button
                    type="button"
                    class="size-7 grid place-items-center rounded-md text-slate-400 hover:text-red-500 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors cursor-pointer bg-transparent border-none shrink-0"
                    aria-label="Remove salary floor"
                    onclick={() => removeSalaryRow(i)}
                  >{@html iconSvg('close', 15)}</button>
                </div>
              {/each}
              <button
                type="button"
                class="inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
                onclick={addSalaryRow}
              >{@html iconSvg('plus', 14)} Add salary floor</button>
            </div>
          </Field>
        </div>
        {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
          {#if briefData.constraints.visa_sponsorship}
            <div>
              <label class="wp-label">Visa Sponsorship</label>
              <div class="text-sm text-slate-700 dark:text-slate-200">{optionLabel(briefData.constraints.visa_sponsorship, VISA_OPTIONS)}</div>
            </div>
          {/if}
          {#if (briefData.constraints.salary_floor ?? []).length}
            <div>
              <label class="wp-label">Salary Floor</label>
              <div class="space-y-1">
                {#each briefData.constraints.salary_floor as f}
                  <div class="text-sm text-slate-700 dark:text-slate-200">{floorLine(f)}</div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
        {#if !briefData.constraints.visa_sponsorship && !(briefData.constraints.salary_floor ?? []).length}
          <p class="text-sm text-slate-400 dark:text-slate-500">No constraints set yet.</p>
        {/if}
        {/if}
      </section>
    </Card>
  {:else if api.brief.loading}
    <Spinner text="Loading brief..." />
  {/if}
  {/if}
</div>
