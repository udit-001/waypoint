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
  import { prettify } from '../lib/brief.js';
  import * as api from '../stores/api.svelte.js';

  let profileData = $state(null);
  let briefData = $state(null);

  let saving = $state(false);
  let saveError = $state(null);
  let saveOk = $state(false);
  let saveOkTimer = null;

  // Editable salary-floor rows (free-text inputs commit on blur).
  let salaryRows = $state([]);

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

    await Promise.all([api.profile.ensure(), api.brief.ensure()]);
    profileData = api.profile.value;
    briefData = api.brief.value;
    syncSalaryRows();
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
  <!-- Job Search Preferences -->
  {#if briefData}
    <Card hover={false}>
      <div class="flex items-start justify-between mb-6">
        <div>
          <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200">
            {@html iconSvg('target', 18)} Job Search Preferences
          </h3>
          <p class="text-xs text-slate-400 mt-2">
            {briefData.complete
              ? 'All set — ready to search.'
              : `${briefData.open.length} preference${briefData.open.length === 1 ? '' : 's'} still to set.`}
          </p>
        </div>
        <div class="flex items-center gap-2 text-xs shrink-0">
          {#if saving}
            <span class="text-slate-400">Saving…</span>
          {:else if saveOk}
            <span class="text-emerald-600 dark:text-emerald-400">Saved ✓</span>
          {/if}
          {#if saveError}
            <span class="text-red-600 dark:text-red-400">{saveError}</span>
          {/if}
        </div>
      </div>

      <!-- Preferences — primary, chip-editable -->
      <section>
        <h4 class="wp-section-title">Preferences</h4>
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
      </section>

      <div class="my-6 border-t border-slate-100 dark:border-slate-700" />

      <!-- Constraints — editable -->
      <section>
        <h4 class="wp-section-title">Constraints</h4>
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
      </section>
    </Card>
  {:else if api.brief.loading}
    <Spinner text="Loading brief..." />
  {/if}

  {#if profileData}
    <!-- Personal Info -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg("user", 18)} Personal Info
      </h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-5">
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
      </div>
    </Card>

    <!-- Skills -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg('zap', 18)} Skills
      </h3>
      <ChipInput value={profileData.skills} placeholder="e.g. Go, React, AWS" onchange={(v) => save({ skills: v })} />
    </Card>

    <!-- Experience -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg('briefcase', 18)} Experience
      </h3>
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
    </Card>

    <!-- Education -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-4">
        {@html iconSvg('grad', 18)} Education
      </h3>
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
    </Card>
  {:else if api.profile.loading}
    <Spinner text="Loading profile..." />
  {:else}
    <p class="text-sm text-slate-400">Profile not loaded (server may be down).</p>
  {/if}
</div>
