<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import ZenModelPicker from '../components/ZenModelPicker.svelte';
  import Card from '../components/Card.svelte';
  import * as api from '../stores/api.svelte.js';
  import { chipState, verdictMeta, dotColor, shortWhen, scheduleStatus, clockTime, progressLine, CHIP_TONE, DOT_TONE } from '../lib/autopilotCard.js';

  // WP-183: one tabbed autopilot card (locked Harbor variation 4).
  // Always visible: state chip + enable toggle. Tabs carry the rest.
  let tab = $state('runs');

  let autopilot = $state(null);
  let toggling = $state(false);
  let autopilotError = $state(null);

  let scrapers = $state([]);
  let togglingScraper = $state('');

  let togglingDiscovery = $state(false);
  let cadenceSaving = $state(false);

  let discoveredCount = $state(null);

  // Keys: state chips in Setup (artifact). A missing key's chip opens
  // the inline editor row — the one affordance the chip state needs.
  let keyEditor = $state('');
  let zenKeyValue = $state('');
  let zenKeySet = $state(false);
  let zenKeySaving = $state(false);
  let zenKeyError = $state(null);
  let exaKeyValue = $state('');
  let exaKeySet = $state(false);
  let exaKeySaving = $state(false);
  let exaKeyError = $state(null);

  let zenModel = $state('');
  let zenModels = $state([]);
  let zenModelSaving = $state(false);
  let zenModelSaved = $state(false);

  let settingsData = $state(null);
  let currentFont = $state('sans');
  let briefComplete = $state(null);

  onMount(async () => {
    setPage({ title: 'Settings' });

    await api.settings.ensure();
    settingsData = api.settings.value;
    zenModel = settingsData.zenModel || '';
    currentFont = document.documentElement.dataset.font || localStorage.getItem('waypoint_font') || 'sans';

    // The card reads four stores; each refreshes independently so one
    // slow endpoint can't blank the others.
    try { await api.autopilot.ensure(); autopilot = api.autopilot.value; zenKeySet = !!autopilot?.zenKeySet; exaKeySet = !!autopilot?.exaKeySet; } catch { autopilot = null; }

    // Preserve the enable gate the old card carried: autopilot cannot
    // turn on until the curation brief is complete (server enforces it
    // too; this keeps the toggle honest before the attempt).
    try {
      const b = await (await fetch('/api/brief')).json();
      briefComplete = !!b.complete;
    } catch { briefComplete = false; }
    try { await api.scrapers.ensure(); scrapers = api.scrapers.value; } catch { scrapers = []; }
    try { await api.companies.ensure(); discoveredCount = (api.companies.value || []).filter((c) => c.discovered).length; } catch { discoveredCount = null; }

    try {
      const res = await fetch('/api/zen/models');
      if (res.ok) zenModels = (await res.json()).models || [];
    } catch { /* dropdown falls back to current selection only */ }
  });

  const pendingShortlists = $derived(autopilot?.pendingShortlists ?? 0);
  const lastRun = $derived(autopilot?.lastRun ?? null);
  const chip = $derived(autopilot ? chipState(!!autopilot.enabled, lastRun?.verdict ?? '', pendingShortlists) : null);
  const schedule = $derived(scheduleStatus(autopilot));
  const progress = $derived(progressLine(autopilot));
  const discoveryOn = $derived(settingsData ? settingsData.discoveryEnabled !== false : true);
  const discoveryInterval = $derived(settingsData?.discoveryIntervalDays || 30);
  const cadence = $derived(autopilot?.cadence || 6);
  const enableBlocked = $derived(briefComplete === false && !autopilot?.enabled);

  // The verdict line is the hero run; the rows beneath are the NEXT 4
  // (trust over time, no duplicate hero row).
  const hero = $derived(lastRun ? verdictMeta(lastRun) : null);
  const recentRuns = $derived(
    (autopilot?.runs || []).slice(1, 5).map((r) => ({ meta: verdictMeta(r), verdict: r.verdict, when: shortWhen(r.startedAt) })),
  );
  const noRunMeta = $derived(verdictMeta(null));

  const chipTone = CHIP_TONE;
  const dotTone = DOT_TONE;

  async function toggleAutopilot() {
    if (toggling) return;
    toggling = true;
    autopilotError = null;
    const newState = !autopilot.enabled;
    try {
      await api.saveSetting('autopilot_enabled', newState ? 1 : 0);
      // The daemon polls every 30s: a fresh enable means the first (or an
      // immediate requested) run — reflect it without a refetch.
      autopilot = { ...autopilot, enabled: newState, firstRun: newState, running: false };
    } catch (e) {
      autopilotError = e.message;
    } finally {
      toggling = false;
    }
  }

  // One scraper's opt-out flips one entry in the disabled list; the
  // PATCH replaces the whole canonical array (the WP-175 whitelist).
  async function toggleScraper(s) {
    if (togglingScraper) return;
    togglingScraper = s.id;
    try {
      const disabledIds = scrapers.filter((x) => x.disabled && x.id !== s.id).map((x) => x.id);
      const next = s.disabled ? disabledIds : [...disabledIds, s.id];
      await api.saveSetting('autopilot_disabled_scrapers', next);
      scrapers = scrapers.map((x) => (x.id === s.id ? { ...x, disabled: !x.disabled } : x));
    } catch (e) {
      autopilotError = e.message;
    } finally {
      togglingScraper = '';
    }
  }

  async function toggleDiscovery() {
    if (togglingDiscovery) return;
    togglingDiscovery = true;
    try {
      const next = !discoveryOn;
      await api.saveSetting('discovery_enabled', next ? 1 : 0);
      settingsData = { ...settingsData, discoveryEnabled: next };
    } catch (e) {
      autopilotError = e.message;
    } finally {
      togglingDiscovery = false;
    }
  }

  async function stepCadence(delta) {
    if (cadenceSaving) return;
    const next = Math.min(24, Math.max(1, cadence + delta));
    if (next === cadence) return;
    cadenceSaving = true;
    try {
      await api.saveSetting('autopilot_cadence', next);
      autopilot = { ...autopilot, cadence: next };
    } catch (e) {
      autopilotError = e.message;
    } finally {
      cadenceSaving = false;
    }
  }

  async function saveZenKey() {
    zenKeyError = null;
    if (!zenKeyValue.trim()) {
      zenKeyError = 'Paste a key first.';
      return;
    }
    zenKeySaving = true;
    try {
      await api.saveSetting('zen_api_key', zenKeyValue.trim());
      zenKeySet = true;
      zenKeyValue = '';
      keyEditor = '';
    } catch (e) {
      zenKeyError = e.message;
    } finally {
      zenKeySaving = false;
    }
  }

  async function saveExaKey() {
    exaKeyError = null;
    if (!exaKeyValue.trim()) {
      exaKeyError = 'Paste a key first.';
      return;
    }
    exaKeySaving = true;
    try {
      await api.saveSetting('exa_api_key', exaKeyValue.trim());
      exaKeySet = true;
      exaKeyValue = '';
      keyEditor = '';
    } catch (e) {
      exaKeyError = e.message;
    } finally {
      exaKeySaving = false;
    }
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

  function setFont(font) {
    currentFont = font;
    document.documentElement.dataset.font = font;
    localStorage.setItem('waypoint_font', font);
  }
</script>

<div class="space-y-4">
  <div class="pt-2">
    <h2 class="text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500 px-1">Autopilot</h2>
  </div>

  <Card hover={false} class="!p-4">
    <!-- Always visible: the answer to "does anything need me?" + the one enable toggle -->
    <div class="flex items-center justify-between gap-3">
      <span class="text-sm font-semibold text-slate-800 dark:text-slate-100">Autopilot</span>
      <span class="flex items-center gap-2">
        {#if chip}
          <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-semibold {chipTone[chip.tone]}">{chip.copy}</span>
        {/if}
        <button
          role="switch"
          aria-checked={!!autopilot?.enabled}
          aria-label="Autopilot enabled"
          disabled={toggling || !autopilot || enableBlocked}
          title={enableBlocked ? 'Complete your curation brief first' : ''}
          class="relative inline-flex h-[17px] w-[30px] items-center rounded-full transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 {autopilot?.enabled ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-slate-600'}"
          onclick={toggleAutopilot}
        >
          <span class="inline-block h-[13px] w-[13px] transform rounded-full bg-white transition-transform {autopilot?.enabled ? 'translate-x-[15px]' : 'translate-x-[2px]'}" />
        </button>
      </span>
    </div>
    {#if autopilotError}
      <p class="text-xs text-red-600 mt-2">{autopilotError}</p>
    {/if}
    {#if schedule?.kind !== 'cadence-wait' && schedule}
      <!-- Progress line: a mid-flight or first cycle is the one state the
           runs tab can't show yet (no finished run to list) -->
      <p class="text-xs text-slate-400 dark:text-slate-500 mt-2">
        {schedule.kind === 'run-in-progress'
          ? (progress || 'Run in progress — the Runs list updates when it finishes.')
          : 'First run underway — your first matches land within a minute or two.'}
      </p>
    {/if}

    <!-- Tabs: Runs / Review / Sources / Setup (variation 4) -->
    <div class="flex gap-0.5 bg-slate-100 dark:bg-slate-700 rounded-lg p-[3px] mt-3">
      {#each [['runs', 'Runs'], ['review', 'Review'], ['sources', 'Sources'], ['setup', 'Setup']] as [id, label]}
        <button
          class="flex-1 rounded-md py-1.5 text-xs font-medium cursor-pointer {tab === id
            ? 'bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-100 shadow-sm'
            : 'text-slate-500 dark:text-slate-400'}"
          onclick={() => (tab = id)}
        >{label}</button>
      {/each}
    </div>

    {#if !autopilot}
      <div class="py-4"><Spinner text="Loading autopilot..." /></div>
    {:else if tab === 'runs'}
      <!-- Last-run verdict line + 4 recent runs + the one rule -->
      {#if lastRun}
        <div class="row flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
          <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-semibold {chipTone[hero.chipClass || 'slate']}">{hero.label}</span>
          <span class="text-xs text-slate-500 flex-1 text-left">{hero.what}</span>
        </div>
      {/if}
      {#each recentRuns as r}
        <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
          <span class="flex items-center gap-2 text-[13px] text-slate-700 dark:text-slate-200 shrink-0">
            <span class="inline-block size-2 rounded-full {dotTone[dotColor(r.verdict)]}"></span>
            {r.when}
          </span>
          <span class="text-xs text-slate-500 flex-1 text-left">{r.meta.label} — {r.meta.what}</span>
        </div>
      {:else}
        <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
          <span class="text-[13px] text-slate-700 dark:text-slate-200">Runs</span>
          <span class="text-xs text-slate-500 flex-1 text-left">{noRunMeta.label} — {noRunMeta.what}</span>
        </div>
      {/each}
      <div class="flex items-center min-h-[30px] py-1">
        <span class="text-[11px] text-slate-400 dark:text-slate-500">3 quiet in a row = stale search</span>
      </div>
    {:else if tab === 'review'}
      <!-- The two entry points into the human gates -->
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
        <span class="text-[13px] text-slate-700 dark:text-slate-200">Review queue</span>
        <span class="text-xs text-slate-500 flex-1 text-left">{pendingShortlists} shortlists</span>
        <a href="/found" class="text-xs font-medium text-link hover:underline shrink-0">Open →</a>
      </div>
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
        <span class="text-[13px] text-slate-700 dark:text-slate-200">New boards</span>
        <span class="text-xs text-slate-500 flex-1 text-left">{discoveredCount === null ? '—' : discoveredCount} — keep = ignore, block = gone</span>
        <a href="/companies" class="text-xs font-medium text-link hover:underline shrink-0">Companies →</a>
      </div>
    {:else if tab === 'sources'}
      <!-- Per-scraper opt-outs; off here = autopilot never uses it -->
      {#each scrapers as s (s.id)}
        <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
          <span class="text-[13px] text-slate-700 dark:text-slate-200 shrink-0">{s.name}</span>
          <span class="text-xs text-slate-500 flex-1 text-left">{s.aggregator ? 'aggregator' : 'portal'}</span>
          <button
            role="switch"
            aria-checked={!s.disabled}
            aria-label="{s.name} enabled"
            disabled={togglingScraper === s.id}
            class="relative inline-flex h-[17px] w-[30px] items-center rounded-full transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 {!s.disabled ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-slate-600'}"
            onclick={() => toggleScraper(s)}
          >
            <span class="inline-block h-[13px] w-[13px] transform rounded-full bg-white transition-transform {!s.disabled ? 'translate-x-[15px]' : 'translate-x-[2px]'}" />
          </button>
        </div>
      {/each}
      <!-- Company discovery: one toggle; the interval is the knob's value -->
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px]">
        <span class="text-[13px] text-slate-700 dark:text-slate-200 shrink-0">Discovery (Exa)</span>
        <span class="text-xs text-slate-500 flex-1 text-left">every {discoveryInterval} days</span>
        <button
          role="switch"
          aria-checked={discoveryOn}
          aria-label="Company discovery enabled"
          disabled={togglingDiscovery}
          class="relative inline-flex h-[17px] w-[30px] items-center rounded-full transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 {discoveryOn ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-slate-600'}"
          onclick={toggleDiscovery}
        >
          <span class="inline-block h-[13px] w-[13px] transform rounded-full bg-white transition-transform {discoveryOn ? 'translate-x-[15px]' : 'translate-x-[2px]'}" />
        </button>
      </div>
    {:else if tab === 'setup'}
      <!-- Cadence stepper + keys as state chips -->
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
        <span class="text-[13px] text-slate-700 dark:text-slate-200">Cadence</span>
        <span class="inline-flex items-center border border-slate-200 dark:border-slate-600 rounded-md overflow-hidden shrink-0">
          <button class="w-7 h-7 bg-slate-50 dark:bg-slate-700 text-slate-500 cursor-pointer" aria-label="Decrease cadence" onclick={() => stepCadence(-1)}>−</button>
          <span class="min-w-[74px] text-center text-xs font-medium text-slate-700 dark:text-slate-200">every {cadence}h</span>
          <button class="w-7 h-7 bg-slate-50 dark:bg-slate-700 text-slate-500 cursor-pointer" aria-label="Increase cadence" onclick={() => stepCadence(1)}>+</button>
        </span>
      </div>
      <!-- Zen key row -->
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
        <span class="text-[13px] text-slate-700 dark:text-slate-200">Zen key</span>
        <span class="text-xs text-slate-500 flex-1 text-left">scoring</span>
        {#if keyEditor === 'zen'}
          <span class="flex items-center gap-2 shrink-0">
            <input
              type="password"
              bind:value={zenKeyValue}
              placeholder="oc_…"
              autocomplete="off"
              aria-label="Zen API key"
              maxlength="128"
              class="w-44 px-2 py-1 text-xs bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-md focus:outline-none focus:border-slate-400"
            />
            <button class="px-2 py-1 text-[11px] font-medium bg-slate-800 text-white rounded-md hover:opacity-90 cursor-pointer disabled:opacity-50" disabled={zenKeySaving} onclick={saveZenKey}>{zenKeySaving ? 'Saving…' : 'Save'}</button>
          </span>
        {:else}
          <button
            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-semibold cursor-pointer {zenKeySet ? chipTone.green + ' cursor-default' : chipTone.amber}"
            aria-label={zenKeySet ? 'Zen key set' : 'Add Zen key'}
            onclick={() => (zenKeySet ? null : (keyEditor = 'zen'))}
          >{zenKeySet ? 'ok' : 'missing'}</button>
        {/if}
      </div>
      {#if keyEditor === 'zen' && zenKeyError}
        <p class="text-xs text-red-600">{zenKeyError}</p>
      {/if}
      <!-- Exa key row -->
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px] border-b border-slate-100 dark:border-slate-600">
        <span class="text-[13px] text-slate-700 dark:text-slate-200">Exa key</span>
        <span class="text-xs text-slate-500 flex-1 text-left">discovery</span>
        {#if keyEditor === 'exa'}
          <span class="flex items-center gap-2 shrink-0">
            <input
              type="password"
              bind:value={exaKeyValue}
              placeholder="exa key"
              autocomplete="off"
              aria-label="Exa API key"
              maxlength="128"
              class="w-44 px-2 py-1 text-xs bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-md focus:outline-none focus:border-slate-400"
            />
            <button class="px-2 py-1 text-[11px] font-medium bg-slate-800 text-white rounded-md hover:opacity-90 cursor-pointer disabled:opacity-50" disabled={exaKeySaving} onclick={saveExaKey}>{exaKeySaving ? 'Saving…' : 'Save'}</button>
          </span>
        {:else}
          <button
            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-semibold cursor-pointer {exaKeySet ? chipTone.green + ' cursor-default' : chipTone.amber}"
            aria-label={exaKeySet ? 'Exa key set' : 'Add Exa key'}
            onclick={() => (exaKeySet ? null : (keyEditor = 'exa'))}
          >{exaKeySet ? 'ok' : 'missing'}</button>
        {/if}
      </div>
      {#if keyEditor === 'exa' && exaKeyError}
        <p class="text-xs text-red-600">{exaKeyError}</p>
      {/if}
      <!-- Curation model: shipped in WP-163; kept here until its fate is decided -->
      <div class="flex items-center justify-between gap-3 min-h-[36px] py-[5px]">
        <span class="text-[13px] text-slate-700 dark:text-slate-200">Curation model</span>
        <span class="flex items-center gap-2 shrink-0">
          <ZenModelPicker models={zenModels} bind:value={zenModel} />
          <button class="px-2 py-1 text-[11px] font-medium bg-slate-800 text-white rounded-md hover:opacity-90 cursor-pointer disabled:opacity-50" disabled={zenModelSaving || !zenKeySet} title={zenKeySet ? '' : 'Save a Zen key first'} onclick={saveZenModel}>{zenModelSaving ? 'Saving…' : zenModelSaved ? 'Saved' : 'Apply'}</button>
        </span>
      </div>
    {/if}
  </Card>

  <div class="pt-2">
    <h2 class="text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500 px-1">Preferences</h2>
  </div>

<!-- App Settings -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-2">
      {@html iconSvg('sliders', 20)} App Settings
    </h3>
    {#if settingsData}
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Default view</label>
          <div class="text-sm text-slate-700">{settingsData.defaultView || 'dashboard'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Theme</label>
          <div class="text-sm text-slate-700 capitalize">{settingsData.theme || 'light'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Notifications</label>
          <div class="text-sm text-slate-700">{settingsData.remindersEnabled ? 'Enabled' : 'Disabled'}</div>
        </div>
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Items per page</label>
          <div class="text-sm text-slate-700">{settingsData.itemsPerPage || 25}</div>
        </div>
      </div>
    {:else}
      <Spinner text="Loading settings..." />
    {/if}
  </Card>


<!-- Typography -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-2">
      <span class="text-lg">{@html iconSvg('type', 20)}</span> Typography
    </h3>

    <div class="flex gap-3">
      <button
        class="flex-1 p-4 rounded-lg border-2 text-center cursor-pointer transition-[border-color] {currentFont === 'sans' ? 'border-slate-700 bg-slate-50' : 'border-slate-200 bg-slate-50 hover:border-slate-300'}"
        style="font-family: 'Inter', sans-serif"
        onclick={() => setFont('sans')}
      >
        <div class="text-xl font-semibold mb-1">Aa</div>
        <div class="text-xs opacity-70">Inter</div>
        <div class="text-xs opacity-50 mt-0.5">Sans-serif</div>
      </button>
      <button
        class="flex-1 p-4 rounded-lg border-2 text-center cursor-pointer transition-[border-color] {currentFont === 'serif' ? 'border-slate-700 bg-slate-50' : 'border-slate-200 bg-slate-50 hover:border-slate-300'}"
        style="font-family: 'PT Serif', serif"
        onclick={() => setFont('serif')}
      >
        <div class="text-xl font-semibold mb-1">Aa</div>
        <div class="text-xs opacity-70">PT Serif</div>
        <div class="text-xs opacity-50 mt-0.5">Serif</div>
      </button>
    </div>
  </Card>


</div>
