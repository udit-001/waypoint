<script>
  // Get Started — inline step-by-step wizard. Composes:
  //   - linkedinImport (lib/linkedinImport.svelte.js) for profile setup
  //   - api.saveSetting for zen/exa/autopilot (single seam, three callers)
  //   - wizard nav stays here (it's pure UI state, belongs in the view)
  import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import * as api from '../stores/api.svelte.js';
  import { getRouter } from '../stores/router.svelte.js';
  import { setup, dismissSetup } from '../lib/onboarding.svelte.js';
  import { createLinkedInImport } from '../lib/linkedinImport.svelte.js';
  import { briefStatus } from '../lib/brief.js';
  import Spinner from '../components/Spinner.svelte';
  import EntryEditor from '../components/EntryEditor.svelte';
  import ChipInput from '../components/ChipInput.svelte';
  import SelectInput from '../components/SelectInput.svelte';
  import Field from '../components/Field.svelte';

  const router = getRouter();
  const PILL_CLS = 'inline-flex items-center bg-slate-100 dark:bg-slate-700 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-200 rounded-full px-2.5 py-0.5 text-xs';

  // ── Wizard state ────────────────────────────────────────────────────
  let currentStep = $state(parseInt(new URLSearchParams(window.location.search).get('step') || '0', 10) || 0);
  $effect(() => {
    const url = new URL(window.location.href);
    url.searchParams.set('step', String(currentStep));
    window.history.replaceState({}, '', url);
  });

  // Composed: LinkedIn import (fetch → preview → apply)
  const linkedin = createLinkedInImport(api);

  // Step 2 — job preferences (brief config)
  let briefData = $state(null);
  const prefsStatus = $derived(briefStatus(briefData));
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
  // Editable salary-floor rows
  let salaryRows = $state([]);

  // Composed: settings via single seam
  let zenKeySaving = $state(false);
  let zenKeySaved = $state(false);
  let zenKeyError = $state(null);
  let zenKeySet = $state(false);
  let zenKeyValue = $state('');

  let exaKeySaving = $state(false);
  let exaKeySaved = $state(false);
  let exaKeyError = $state(null);
  let exaKeySet = $state(false);
  let exaKeyValue = $state('');

  let zenModel = $state('');
  let zenModels = $state([]);
  let zenModelSaving = $state(false);
  let zenModelSaved = $state(false);

  let autopilotEnabled = $state(false);
  let autopilotToggling = $state(false);

  const TOTAL_STEPS = 5;

  // ── Derived ─────────────────────────────────────────────────────────
  const profileImported = $derived(
    linkedin.applied || (!!api.profile.value?.name && ((api.profile.value?.skills?.length ?? 0) > 0 || (api.profile.value?.experience?.length ?? 0) > 0))
  );

  const steps = $derived([
    { id: 'linkedin', label: 'Profile', done: profileImported },
    { id: 'prefs', label: 'Preferences', done: prefsStatus.complete },
    { id: 'zen', label: 'Scoring', done: zenKeySet },
    { id: 'exa', label: 'Discovery', done: exaKeySet },
    { id: 'autopilot', label: 'Autopilot', done: autopilotEnabled },
  ]);

  const stepDoneCount = $derived(steps.filter(s => s.done).length);

  // ── Effects ─────────────────────────────────────────────────────────
  $effect(() => {
    setPage({ title: 'Get started', byline: `${stepDoneCount} of ${TOTAL_STEPS} done` });
  });

  $effect(() => {
    if (setup.dismissed) router.navigate('/applications');
  });

  // ── Init ────────────────────────────────────────────────────────────
  async function init() {
    await Promise.all([
      api.profile.ensure().catch(() => {}),
      api.brief.ensure().catch(() => {}),
    ]);
    briefData = api.brief.value;
    syncSalaryRows();

    try {
      const data = await api.autopilot.ensure().then(() => api.autopilot.value);
      zenKeySet = !!data?.zenKeySet;
      exaKeySet = !!data?.exaKeySet;
      autopilotEnabled = !!data?.enabled;
    } catch {}

    // Load current zen model + available models
    try {
      await api.settings.ensure();
      zenModel = api.settings.value?.zenModel || 'mimo-v2.5-free';
    } catch {}
    loadZenModels();
  }
  init();

  // ── Preferences actions ─────────────────────────────────────────────
  const BRIEF_PREF_KEYS = {
    locationPreference: 'location_preference',
    companies: 'companies',
    avoidCompanies: 'avoid_companies',
    keywords: 'keywords',
    dealbreakers: 'dealbreakers',
  };

  function syncSalaryRows() {
    const floors = briefData?.constraints?.salary_floor || [];
    salaryRows = floors.map((f) => ({ region: f.region || '', amount: String(f.amount ?? '') }));
  }

  async function savePrefs(fields) {
    try {
      briefData = await api.updateBrief(fields);
    } catch { /* save failed silently */ }
  }

  function setRemote(value) {
    if (briefData) briefData.preferences.remote = value;
    savePrefs({ remote: value });
  }

  function setVisa(value) {
    if (briefData) briefData.constraints.visa_sponsorship = value;
    savePrefs({ visaSponsorship: value });
  }

  function setList(key, value) {
    const briefKey = BRIEF_PREF_KEYS[key];
    if (briefData && briefKey) briefData.preferences[briefKey] = value;
    savePrefs({ [key]: value });
  }

  function commitSalaryRows() {
    const floors = salaryRows
      .map((r) => ({ region: r.region.trim(), amount: Number(r.amount) }))
      .filter((f) => f.region !== '' && Number.isFinite(f.amount) && f.amount > 0);
    savePrefs({ salaryFloor: floors });
  }

  function addSalaryRow() {
    salaryRows = [...salaryRows, { region: '', amount: '' }];
  }

  function removeSalaryRow(i) {
    salaryRows = salaryRows.filter((_, idx) => idx !== i);
    commitSalaryRows();
  }

  // ── Settings actions (all go through the same seam) ─────────────────
  async function saveZenKey() {
    if (!zenKeyValue.trim() || zenKeySaving) return;
    zenKeyError = null;
    zenKeySaving = true;
    try {
      await api.saveSetting('zen_api_key', zenKeyValue.trim());
      zenKeySet = true;
      zenKeyValue = '';
      zenKeySaved = true;
      setTimeout(() => { zenKeySaved = false; }, 2000);
    } catch (e) {
      zenKeyError = e.message;
    }
    zenKeySaving = false;
  }

  async function saveExaKey() {
    if (!exaKeyValue.trim() || exaKeySaving) return;
    exaKeyError = null;
    exaKeySaving = true;
    try {
      await api.saveSetting('exa_api_key', exaKeyValue.trim());
      exaKeySet = true;
      exaKeyValue = '';
      exaKeySaved = true;
      setTimeout(() => { exaKeySaved = false; }, 2000);
    } catch (e) {
      exaKeyError = e.message;
    }
    exaKeySaving = false;
  }

  async function loadZenModels() {
    try {
      const res = await fetch('/api/zen/models');
      if (res.ok) {
        const data = await res.json();
        zenModels = data.models || [];
      }
    } catch { /* dropdown falls back to current selection only */ }
  }

  async function saveZenModel() {
    if (zenModelSaving) return;
    zenModelSaving = true;
    try {
      await api.saveSetting('zen_model', zenModel);
      zenModelSaved = true;
      setTimeout(() => { zenModelSaved = false; }, 1500);
    } finally {
      zenModelSaving = false;
    }
  }

  async function toggleAutopilot() {
    if (autopilotToggling) return;
    autopilotToggling = true;
    try {
      await api.saveSetting('autopilot_enabled', autopilotEnabled ? 0 : 1);
      autopilotEnabled = !autopilotEnabled;
    } catch {}
    autopilotToggling = false;
  }

  // ── Navigation ──────────────────────────────────────────────────────
  function nextStep() {
    if (currentStep < TOTAL_STEPS - 1) currentStep++;
  }

  function prevStep() {
    if (currentStep > 0) currentStep--;
  }

  function finish() {
    dismissSetup();
    router.navigate('/applications');
  }
</script>

<div class="max-w-3xl mx-auto py-10 px-4">
  <!-- Progress bar -->
  <!-- Stepper: circles with labels below, connected by lines. Matches the
       reference design — no overflow issues at 5 steps. -->
  <!-- Stepper: circles with labels below, connected by lines.
       Uses high-contrast colors against the page background so every
       circle is visible regardless of theme. -->
  <div class="flex items-start justify-center mb-8">
    {#each steps as step, i (step.id)}
      <div class="flex flex-col items-center">
        <button
          class="flex items-center justify-center rounded-full transition-colors cursor-pointer border-none p-0
            size-[28px] text-xs font-semibold mb-1.5
            {i === currentStep
              ? 'bg-slate-800 text-white ring-2 ring-slate-500 ring-offset-2 ring-offset-slate-100'
              : step.done
                ? 'bg-emerald-700 text-white'
                : 'bg-slate-500 text-slate-50'}"
          onclick={() => { currentStep = i; }}
          aria-label="Step {i + 1}: {step.label}"
        >
          {#if step.done}
            {@html iconSvg('check', 14)}
          {:else}
            {i + 1}
          {/if}
        </button>
        <span class="text-[11px] font-medium whitespace-nowrap
          {i === currentStep ? 'text-slate-900' : 'text-slate-600'}">
          {step.label}
        </span>
      </div>
      {#if i < steps.length - 1}
        <div class="flex-1 h-px mt-[14px] mx-1
          {step.done ? 'bg-emerald-700' : 'bg-slate-500'}"></div>
      {/if}
    {/each}
  </div>

  <!-- Step content -->
  <div class="min-h-[380px]">
    {#if currentStep === 0}
      <!-- ═══ Step 1: LinkedIn import ═══ -->
      <div>
        <h3 class="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">Import from LinkedIn</h3>
        <p class="text-sm text-slate-500 dark:text-slate-300 mb-5">Paste your public profile URL — we'll prefill name, title, skills, experience, and education.</p>

        {#if linkedin.applied}
          <div class="flex items-center gap-2 p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/60 rounded-xl mb-4">
            <span class="text-emerald-800 dark:text-emerald-300">{@html iconSvg('check-circle', 16)}</span>
            <span class="text-sm text-emerald-800 dark:text-emerald-200">
              Profile imported{linkedin.appliedSummary ? ` — ${linkedin.appliedSummary}` : ''}.
            </span>
          </div>
        {/if}

        {#if profileImported && !linkedin.applied}
          <div class="flex items-center gap-2 p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/60 rounded-xl mb-4">
            <span class="text-emerald-800 dark:text-emerald-300">{@html iconSvg('check-circle', 16)}</span>
            <span class="text-sm text-emerald-800 dark:text-emerald-200">Profile loaded — {api.profile.value?.name || 'unnamed'}.</span>
          </div>
        {/if}

        {#if linkedin.preview}
          <!-- Diff preview -->
          <div class="space-y-4">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-3">
              {#if linkedin.preview.doc.name}
                <div><label class="wp-label">Name</label><div class="text-sm text-slate-700 dark:text-slate-200">{linkedin.preview.doc.name}</div></div>
              {/if}
              {#if linkedin.preview.doc.title}
                <div><label class="wp-label">Title</label><div class="text-sm text-slate-700 dark:text-slate-200">{linkedin.preview.doc.title}</div></div>
              {/if}
              {#if linkedin.preview.doc.currentLocation}
                <div><label class="wp-label">Location</label><div class="text-sm text-slate-700 dark:text-slate-200">{linkedin.preview.doc.currentLocation}</div></div>
              {/if}
            </div>
            {#if (linkedin.preview.summary.skillsAdded ?? []).length}
              <div>
                <label class="wp-label">{profileImported ? 'New skills' : 'Skills'}</label>
                <div class="flex flex-wrap gap-1.5">
                  {#each linkedin.preview.summary.skillsAdded as s}
                    <span class={PILL_CLS}>{s}</span>
                  {/each}
                </div>
              </div>
            {/if}
            {#if (linkedin.preview.summary.experienceAdded ?? []).length}
              <div>
                <label class="wp-label">{profileImported ? 'New experience' : 'Experience'}</label>
                <EntryEditor entries={linkedin.preview.summary.experienceAdded} primaryKey="title" secondaryKey="company" readonly />
              </div>
            {/if}
            {#if (linkedin.preview.summary.educationAdded ?? []).length}
              <div>
                <label class="wp-label">{profileImported ? 'New education' : 'Education'}</label>
                <EntryEditor entries={linkedin.preview.summary.educationAdded} primaryKey="degree" secondaryKey="institution" readonly />
              </div>
            {/if}
            <div class="flex items-center gap-3">
              <button
                class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium bg-slate-800 dark:bg-slate-200 text-white dark:text-slate-800 rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50"
                disabled={linkedin.fetching}
                onclick={() => linkedin.apply()}
              >{@html iconSvg('check', 15)} {linkedin.fetching ? 'Applying…' : 'Apply to profile'}</button>
              <button
                class="text-sm text-slate-500 dark:text-slate-300 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
                onclick={() => linkedin.discard()}
              >Discard</button>
            </div>
            <p class="text-xs text-slate-400 dark:text-slate-300">
              {profileImported
                ? 'Adds new roles and skills, updates matched ones, keeps everything else.'
                : 'Applying replaces current name, title, location, skills, experience, and education.'}
            </p>
          </div>
        {:else}
          <!-- URL input -->
          <div class="flex gap-2">
            <input
              class="wp-input flex-1 min-w-0 placeholder:text-slate-400"
              placeholder="https://www.linkedin.com/in/username"
              aria-label="LinkedIn profile URL"
              bind:value={linkedin.url}
              onkeydown={(e) => { if (e.key === 'Enter' && !linkedin.fetching && linkedin.url.trim()) linkedin.fetchProfile(); }}
            />
            <button
              class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium bg-slate-800 dark:bg-slate-200 text-white dark:text-slate-800 rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50 shrink-0"
              disabled={linkedin.fetching || !linkedin.url.trim()}
              onclick={() => linkedin.fetchProfile()}
            >{@html iconSvg('linkedin', 15)} {linkedin.fetching ? 'Fetching…' : 'Fetch'}</button>
          </div>
          {#if linkedin.fetching}
            <p class="text-xs text-slate-400 dark:text-slate-300 mt-2">Fetching profile — usually 5–15 seconds.</p>
          {:else}
            <p class="text-xs text-slate-500 dark:text-slate-300 mt-2">Profile must be public.</p>
          {/if}
        {/if}

        {#if linkedin.error}
          <p class="text-xs text-red-600 dark:text-red-400 mt-2">{linkedin.error}</p>
        {/if}
      </div>

    {:else if currentStep === 1}
      <!-- ═══ Step 2: Job preferences ═══ -->
      <div>
        <h3 class="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">Job preferences</h3>
        <p class="text-sm text-slate-500 dark:text-slate-300 mb-5">What kind of job are you looking for? These drive the search — every posting is judged against them.</p>

        {#if briefData}
          <div class="space-y-5">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-4">
              <SelectInput
                label="Remote"
                value={briefData.preferences.remote}
                options={REMOTE_OPTIONS}
                onchange={setRemote}
              />
              <SelectInput
                label="Visa Sponsorship"
                value={briefData.constraints.visa_sponsorship}
                options={VISA_OPTIONS}
                onchange={setVisa}
              />
            </div>
            <Field label="Location Preference">
              <ChipInput
                value={briefData.preferences.location_preference}
                placeholder="e.g. Bengaluru, Remote"
                onchange={(v) => setList('locationPreference', v)}
              />
            </Field>
            <Field label="Companies">
              <ChipInput
                value={briefData.preferences.companies}
                placeholder="e.g. Acme, Globex"
                onchange={(v) => setList('companies', v)}
              />
            </Field>
            <Field label="Avoid Companies">
              <ChipInput
                value={briefData.preferences.avoid_companies}
                placeholder="e.g. Enron"
                onchange={(v) => setList('avoidCompanies', v)}
              />
            </Field>
            <Field label="Keywords">
              <ChipInput
                value={briefData.preferences.keywords}
                placeholder="e.g. Go, Distributed systems"
                onchange={(v) => setList('keywords', v)}
              />
            </Field>
            <Field label="Dealbreakers">
              <ChipInput
                value={briefData.preferences.dealbreakers}
                placeholder="e.g. Night shifts, Travel"
                onchange={(v) => setList('dealbreakers', v)}
              />
            </Field>
            <div>
              <label class="wp-label">Salary Floor</label>
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
                  class="inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-300 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
                  onclick={addSalaryRow}
                >{@html iconSvg('plus', 14)} Add salary floor</button>
              </div>
            </div>
          </div>
        {:else}
          <Spinner text="Loading preferences…" />
        {/if}
      </div>

    {:else if currentStep === 2}
      <!-- ═══ Step 3: Zen key ═══ -->
      <div>
        <h3 class="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">Connect scoring</h3>
        <p class="text-sm text-slate-500 dark:text-slate-300 mb-5">A free Zen key makes autopilot judge every posting before it reaches Matches.</p>

        {#if zenKeySet}
          <div class="flex items-center gap-2 p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/60 rounded-xl mb-4">
            <span class="text-emerald-800 dark:text-emerald-300">{@html iconSvg('check-circle', 16)}</span>
            <span class="text-sm text-emerald-800 dark:text-emerald-200">Key saved — scoring is active.</span>
          </div>
        {/if}

        <div>
          <label class="wp-label" for="gs-zen-key">Zen API key</label>
          <div class="flex gap-2">
            <input
              id="gs-zen-key"
              type="password"
              class="wp-input flex-1 min-w-0"
              placeholder={zenKeySet ? 'Replace saved key…' : 'oc_…'}
              bind:value={zenKeyValue}
              onkeydown={(e) => { if (e.key === 'Enter' && zenKeyValue.trim()) saveZenKey(); }}
            />
            <button
              class="px-4 py-2 text-sm font-medium bg-slate-800 dark:bg-slate-200 text-white dark:text-slate-800 rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50 shrink-0"
              disabled={zenKeySaving || !zenKeyValue.trim()}
              onclick={saveZenKey}
            >
              {#if zenKeySaving}Saving…{:else if zenKeySaved}Saved{:else}Save{/if}
            </button>
          </div>
          {#if zenKeyError}
            <p class="text-xs text-red-600 dark:text-red-400 mt-2">{zenKeyError}</p>
          {/if}
          <p class="text-xs text-slate-400 dark:text-slate-300 mt-3 leading-relaxed">
            Sign up at <a href="https://opencode.ai/auth" target="_blank" rel="noopener noreferrer" class="text-link underline">opencode.ai/auth</a> → open <span class="font-medium">Billing</span> → copy your key (starts with <code class="bg-slate-100 dark:bg-slate-800 px-1 rounded">oc_</code>). Uses free-tier models.
          </p>
        </div>

        {#if zenKeySet}
          <div class="mt-4 pt-4 border-t border-slate-100 dark:border-slate-600">
            <label class="wp-label" for="gs-zen-model">Curation model</label>
            <div class="flex gap-2">
              <select
                id="gs-zen-model"
                bind:value={zenModel}
                class="flex-1 min-w-0 px-3 py-2 text-sm bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-lg focus:outline-none focus:border-slate-400"
              >
                <option value="">Default (mimo-v2.5-free)</option>
                {#each zenModels as m}
                  <option value={m}>{m}</option>
                {/each}
                {#if zenModel && !zenModels.includes(zenModel)}
                  <option value={zenModel}>{zenModel}</option>
                {/if}
              </select>
              <button
                class="px-3 py-2 text-xs font-medium bg-slate-800 text-white rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50"
                disabled={zenModelSaving}
                onclick={saveZenModel}
              >{zenModelSaving ? 'Saving…' : zenModelSaved ? 'Saved' : 'Apply'}</button>
            </div>
          </div>
        {/if}
      </div>

    {:else if currentStep === 3}
      <!-- ═══ Step 4: Exa key ═══ -->
      <div>
        <h3 class="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">Enable discovery</h3>
        <p class="text-sm text-slate-500 dark:text-slate-300 mb-5">Optional — suggests new companies to watch based on your profile.</p>

        {#if exaKeySet}
          <div class="flex items-center gap-2 p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/60 rounded-xl mb-4">
            <span class="text-emerald-800 dark:text-emerald-300">{@html iconSvg('check-circle', 16)}</span>
            <span class="text-sm text-emerald-800 dark:text-emerald-200">Key saved — discovery is active.</span>
          </div>
        {/if}

        <div>
          <label class="wp-label" for="gs-exa-key">Exa API key</label>
          <div class="flex gap-2">
            <input
              id="gs-exa-key"
              type="password"
              class="wp-input flex-1 min-w-0"
              placeholder={exaKeySet ? 'Replace saved key…' : 'exa key'}
              bind:value={exaKeyValue}
              onkeydown={(e) => { if (e.key === 'Enter' && exaKeyValue.trim()) saveExaKey(); }}
            />
            <button
              class="px-4 py-2 text-sm font-medium bg-slate-800 dark:bg-slate-200 text-white dark:text-slate-800 rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50 shrink-0"
              disabled={exaKeySaving || !exaKeyValue.trim()}
              onclick={saveExaKey}
            >
              {#if exaKeySaving}Saving…{:else if exaKeySaved}Saved{:else}Save{/if}
            </button>
          </div>
          {#if exaKeyError}
            <p class="text-xs text-red-600 dark:text-red-400 mt-2">{exaKeyError}</p>
          {/if}
          <p class="text-xs text-slate-400 dark:text-slate-300 mt-3 leading-relaxed">
            Create a free account at <a href="https://dashboard.exa.ai" target="_blank" rel="noopener noreferrer" class="text-link underline">dashboard.exa.ai</a> (no credit card needed), then copy a key from <span class="font-medium">API Keys</span>.
          </p>
        </div>
      </div>

    {:else if currentStep === 4}
      <!-- ═══ Step 5: Autopilot ═══ -->
      <div>
        <h3 class="text-lg font-semibold text-slate-800 dark:text-slate-100 mb-1">Turn on autopilot</h3>
        <p class="text-sm text-slate-500 dark:text-slate-300 mb-5">Autopilot runs every 6 hours, finds new postings, scores them, and files the best in Matches.</p>

        {#if prefsStatus.complete === false && !autopilotEnabled}
          <div class="flex items-start gap-2 p-3 bg-tint-amber border border-warning rounded-lg mb-4">
            <span class="text-warning-strong mt-0.5">{@html iconSvg('alert-circle', 14)}</span>
            <div class="text-xs text-warning leading-relaxed">
              <span class="font-medium text-warning-strong">Curation brief incomplete</span> — complete your preferences (location, remote, salary, etc.) before enabling autopilot. Go back to step 2 to finish.
            </div>
          </div>
        {/if}

        <div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-xl">
          <div>
            <p class="text-sm font-medium text-slate-800 dark:text-slate-100">Autopilot</p>
            <p class="text-xs text-slate-400 dark:text-slate-300 mt-0.5">
              {autopilotEnabled ? 'Running every 6 hours' : 'Currently off'}
            </p>
          </div>
          <button
            role="switch"
            aria-checked={!!autopilotEnabled}
            aria-label="Autopilot enabled"
            disabled={autopilotToggling || (prefsStatus.complete === false && !autopilotEnabled)}
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-[120ms] ease-[var(--ease-in-out)] cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 {autopilotEnabled ? 'bg-slate-800' : 'bg-slate-300'}"
            onclick={toggleAutopilot}
          >
            <span
              class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-[120ms] ease-[var(--ease-in-out)] active:scale-90 {autopilotEnabled ? 'translate-x-6' : 'translate-x-1'}"
            />
          </button>
        </div>

        {#if !zenKeySet}
          <p class="text-xs text-amber-600 dark:text-amber-400 mt-3 flex items-center gap-1.5">
            {@html iconSvg('alert-triangle', 14)}
            Without a Zen key, the 25 freshest postings per cycle arrive unscored.
          </p>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Navigation -->
  <div class="flex items-center justify-between mt-6 pt-4 border-t border-slate-100 dark:border-slate-700">
    <div>
      {#if currentStep > 0}
        <button
          class="text-sm text-slate-500 dark:text-slate-300 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
          onclick={prevStep}
        >← Back</button>
      {/if}
    </div>

    <div class="flex items-center gap-4">
      <span class="text-xs text-slate-400 dark:text-slate-300 tabular-nums">
        {currentStep + 1} of {TOTAL_STEPS}
      </span>

      {#if currentStep < TOTAL_STEPS - 1}
        <button
          class="text-sm font-medium text-slate-800 dark:text-slate-100 hover:underline cursor-pointer bg-transparent border-none p-0"
          onclick={nextStep}
        >Next →</button>
      {:else}
        <button
          class="text-sm font-medium bg-slate-800 dark:bg-slate-200 text-white dark:text-slate-800 px-4 py-1.5 rounded-lg hover:opacity-90 transition-colors cursor-pointer"
          onclick={finish}
        >Done</button>
      {/if}

      <button
        class="text-xs text-slate-400 dark:text-slate-300 hover:text-slate-600 dark:hover:text-slate-300 transition-colors cursor-pointer bg-transparent border-none p-0"
        onclick={dismissSetup}
      >Skip</button>
    </div>
  </div>
</div>
