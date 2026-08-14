<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import Field from '../components/Field.svelte';
  import ChipInput from '../components/ChipInput.svelte';
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
    save({ visa_sponsorship: value });
  }

  function setRemote(value) {
    if (briefData) briefData.preferences.remote = value;
    save({ remote: value });
  }

  function setList(key, value) {
    if (briefData) briefData.preferences[key] = value;
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
    save({ salary_floor: floors });
  }

  function esc(str) {
    if (!str) return '-';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
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
          <Field label="Remote">
            <select
              class="wp-select"
              value={briefData.preferences.remote}
              onchange={(e) => setRemote(e.currentTarget.value)}
            >
              {#each REMOTE_OPTIONS as opt}
                <option value={opt.value}>{opt.label}</option>
              {/each}
            </select>
          </Field>
          <Field label="Location Preference">
            <ChipInput
              value={briefData.preferences.location_preference}
              placeholder="e.g. Bengaluru, Remote"
              {prettify}
              onchange={(v) => setList('location_preference', v)}
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
              onchange={(v) => setList('avoid_companies', v)}
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
          <Field label="Visa Sponsorship">
            <select
              class="wp-select"
              value={briefData.constraints.visa_sponsorship}
              onchange={(e) => setVisa(e.currentTarget.value)}
            >
              {#each VISA_OPTIONS as opt}
                <option value={opt.value}>{opt.label}</option>
              {/each}
            </select>
          </Field>
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

      <div class="my-6 border-t border-slate-100 dark:border-slate-700" />

      <!-- Facts — supporting, read-only (seed-edited) -->
      <section>
        <h4 class="wp-section-title">Facts</h4>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-x-4 gap-y-5">
          <div>
            <label class="wp-label">Title</label>
            <div class="text-sm text-slate-700 dark:text-slate-200">{briefData.facts.title || '-'}</div>
          </div>
          <div>
            <label class="wp-label">Seniority</label>
            <div class="text-sm text-slate-700 dark:text-slate-200 capitalize">{briefData.facts.seniority || '-'}</div>
          </div>
          <div>
            <label class="wp-label">Current Location</label>
            <div class="text-sm text-slate-700 dark:text-slate-200">{briefData.facts.current_location || '-'}</div>
          </div>
        </div>
        <div class="mt-5">
          <label class="wp-label">Skills</label>
          {#if briefData.facts.skills?.length}
            <div class="flex flex-wrap gap-1.5">
              {#each briefData.facts.skills as skill}
                <span class="bg-slate-100 dark:bg-slate-700 border border-slate-200 dark:border-slate-600 text-slate-500 dark:text-slate-300 rounded-full px-2.5 py-0.5 text-xs">{skill}</span>
              {/each}
            </div>
          {:else}
            <p class="text-sm text-slate-400">No skills yet.</p>
          {/if}
        </div>
        <p class="text-[11px] text-slate-400 mt-3">Read-only — set from your resume.</p>
      </section>
    </Card>
  {:else if api.brief.loading}
    <Spinner text="Loading brief..." />
  {/if}

  {#if profileData}
    <!-- Personal Info -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-4">
        {@html iconSvg("user", 18)} Personal Info
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Full Name</label>
          <div class="text-sm text-slate-700">{profileData.name || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Professional Title</label>
          <div class="text-sm text-slate-700">{profileData.title || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Email</label>
          <div class="text-sm text-slate-700">{profileData.email || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Phone</label>
          <div class="text-sm text-slate-700">{profileData.phone || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Industry</label>
          <div class="text-sm text-slate-700">{profileData.industry || '-'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Greeting Style</label>
          <div class="text-sm text-slate-700 capitalize">{profileData.greetingStyle || 'formal'}</div>
        </div>
      </div>
    </Card>

    <!-- Skills -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        <span>{@html iconSvg('zap', 18)}</span> Skills
      </h3>
      {#if profileData.skills?.length}
        <div class="flex flex-wrap gap-1.5">
          {#each profileData.skills as skill}
            <span class="bg-slate-100 border border-slate-200 text-slate-500 rounded-full px-2.5 py-0.5 text-xs">{skill}</span>
          {/each}
        </div>
      {:else}
        <p class="text-sm text-slate-400">No skills set yet.</p>
      {/if}
    </Card>

    <!-- Education -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        {@html iconSvg('grad', 18)} Education
      </h3>
      {#if profileData.education?.length}
        <ul class="list-disc pl-5 text-sm text-slate-700 space-y-1">
          {#each profileData.education as item}
            <li>{item}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-slate-400">No education set yet.</p>
      {/if}
    </Card>

    <!-- Experience -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
        {@html iconSvg('briefcase', 18)} Experience
      </h3>
      {#if profileData.experience?.length}
        <ul class="list-disc pl-5 text-sm text-slate-700 space-y-1">
          {#each profileData.experience as item}
            <li>{item}</li>
          {/each}
        </ul>
      {:else}
        <p class="text-sm text-slate-400">No experience set yet.</p>
      {/if}
    </Card>

    <!-- Email Preferences -->
    <Card hover={false}>
      <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-4">
        {@html iconSvg('mail', 18)} Email Preferences
      </h3>
      <div>
        <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Sign-Off</label>
        <div class="text-sm text-slate-700">{profileData.signOff || 'Best regards'}</div>
      </div>
    </Card>
  {:else if api.profile.loading}
    <Spinner text="Loading profile..." />
  {:else}
    <p class="text-sm text-slate-400">Profile not loaded (server may be down).</p>
  {/if}
</div>
