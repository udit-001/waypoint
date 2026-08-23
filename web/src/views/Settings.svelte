<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import { formatDateTime } from '../lib/format.js';
  import * as api from '../stores/api.svelte.js';

  let settingsData = $state(null);
  let currentFont = $state('sans');
  let autopilotEnabled = $state(null);
  let autopilotCadence = $state(6);
  let lastRun = $state(null);
  let autopilotError = $state(null);
  let zenKeySet = $state(false);
  let zenKeyValue = $state('');
  let zenModel = $state('');
  let zenModels = $state([]);
  let zenModelSaving = $state(false);
  let zenModelSaved = $state(false);
  let zenKeySaving = $state(false);
  let zenKeySaved = $state(false);
  let zenKeyError = $state(null);
  let exaKeySet = $state(false);
  let exaKeyValue = $state('');
  let exaKeySaving = $state(false);
  let exaKeySaved = $state(false);
  let exaKeyError = $state(null);
  let autopilotToggling = $state(false);

  onMount(async () => {
    setPage({ title: 'Settings' });

    await api.settings.ensure();
    settingsData = api.settings.value;
    zenModel = settingsData.zenModel || '';
    currentFont = document.documentElement.dataset.font || localStorage.getItem('waypoint_font') || 'sans';

    // Load autopilot data.
    try {
      const res = await fetch('/api/autopilot');
      if (res.ok) {
        const data = await res.json();
        autopilotEnabled = data.enabled;
        autopilotCadence = data.cadence || 6;
        lastRun = data.lastRun;
        zenKeySet = !!data.zenKeySet;
        exaKeySet = !!data.exaKeySet;
      }
    } catch {}
  });

  async function saveExaKey() {
    exaKeyError = null;
    if (!exaKeyValue.trim()) {
      exaKeyError = 'Paste a key first.';
      return;
    }
    exaKeySaving = true;
    try {
      const res = await fetch('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ exa_api_key: exaKeyValue.trim() }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${res.status}`);
      }
      exaKeySet = true;
      exaKeyValue = '';
      exaKeySaved = true;
      setTimeout(() => { exaKeySaved = false; }, 2000);
    } catch (e) {
      exaKeyError = e.message;
    } finally {
      exaKeySaving = false;
    }
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
  loadZenModels();

  async function saveZenModel() {
    if (zenModelSaving) return;
    zenModelSaving = true;
    try {
      const res = await fetch('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ zen_model: zenModel }),
      });
      if (res.ok) {
        zenModelSaved = true;
        setTimeout(() => { zenModelSaved = false; }, 1500);
      }
    } finally {
      zenModelSaving = false;
    }
  }

  async function toggleAutopilot() {
    if (autopilotToggling) return;
    autopilotToggling = true;
    autopilotError = null;
    const newState = !autopilotEnabled;
    try {
      const res = await fetch('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ autopilot_enabled: newState ? 1 : 0 }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        autopilotError = data.error || 'Failed to toggle';
        return;
      }
      const updated = await res.json();
      autopilotEnabled = updated.autopilotEnabled === 1;
    } catch (e) {
      autopilotError = e.message;
    } finally {
      autopilotToggling = false;
    }
  }

  async function saveZenKey() {
    zenKeyError = null;
    if (!zenKeyValue.trim()) {
      zenKeyError = 'Paste a key first — it starts with oc_.';
      return;
    }
    zenKeySaving = true;
    try {
      const res = await fetch('/api/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ zen_api_key: zenKeyValue.trim() }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        zenKeyError = data.error || 'Failed to save key';
        return;
      }
      zenKeySet = true;
      zenKeyValue = '';
      zenKeySaved = true;
      setTimeout(() => { zenKeySaved = false; }, 2000);
    } catch (e) {
      zenKeyError = e.message;
    } finally {
      zenKeySaving = false;
    }
  }

  function runErrors(run) {
    if (!run?.errors) return [];
    try {
      const parsed = JSON.parse(run.errors);
      return Array.isArray(parsed) ? parsed : [];
    } catch { return []; }
  }

  function formatDuration(ms) {
    if (!ms) return '';
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${Math.round(ms / 100) / 10}s`;
    return `${Math.round(ms / 60000)}m`;
  }

  function setFont(font) {
    currentFont = font;
    document.documentElement.dataset.font = font;
    localStorage.setItem('waypoint_font', font);
  }

</script>

<div class="space-y-4">  <div class="pt-2">
    <h2 class="text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500 px-1">Autopilot</h2>
  </div>

<!-- Autopilot -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-2">
      {@html iconSvg('zap', 20)} Autopilot
    </h3>
    {#if autopilotEnabled !== null}
      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <span class="text-sm text-slate-700">Enabled</span>
          <button
            role="switch"
            aria-checked={!!autopilotEnabled}
            aria-label="Autopilot enabled"
            disabled={autopilotToggling}
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-[120ms] ease-[var(--ease-in-out)] cursor-pointer disabled:opacity-50 {autopilotEnabled ? 'bg-slate-800' : 'bg-slate-300'}"
            onclick={toggleAutopilot}
          >
            <span
              class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-[120ms] ease-[var(--ease-in-out)] active:scale-90 {autopilotEnabled ? 'translate-x-6' : 'translate-x-1'}"
            />
          </button>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-sm text-slate-700">Cadence</span>
          <span class="text-sm text-slate-500">Every {autopilotCadence}h</span>
        </div>
        {#if lastRun}
          <div class="flex items-center justify-between">
            <span class="text-sm text-slate-700">Last run</span>
            <span class="text-sm text-slate-500">
              {formatDateTime(lastRun.startedAt) || lastRun.startedAt}
            </span>
          </div>
          {#if lastRun.finishedAt}
            <div class="mt-3 pt-3 border-t border-slate-100 dark:border-slate-600 grid grid-cols-2 gap-y-1.5 text-xs">
              <span class="text-slate-400">Found</span><span class="text-slate-600 dark:text-slate-300 tabular-nums text-right">{lastRun.postingsNew}</span>
              <span class="text-slate-400">Shortlisted</span><span class="text-slate-600 dark:text-slate-300 tabular-nums text-right">{lastRun.postingsShortlisted}</span>
              <span class="text-slate-400">Dismissed</span><span class="text-slate-600 dark:text-slate-300 tabular-nums text-right">{lastRun.postingsDismissed}</span>
              <span class="text-slate-400">Errored</span><span class="text-slate-600 dark:text-slate-300 tabular-nums text-right">{lastRun.postingsErrored}</span>
              <span class="text-slate-400">Duration</span><span class="text-slate-600 dark:text-slate-300 tabular-nums text-right">{formatDuration(lastRun.durationMs)}</span>
            </div>
            {#if lastRun.postingsErrored > 0 && runErrors(lastRun).length > 0}
              <div class="mt-3 pt-3 border-t border-slate-100 dark:border-slate-600">
                <p class="text-[11px] font-semibold uppercase tracking-wide text-red-500 mb-1.5">Last run errors</p>
                <ul class="space-y-1">
                  {#each runErrors(lastRun).slice(0, 5) as err}
                    <li class="text-xs text-red-500 dark:text-red-400 break-words">{err}</li>
                  {/each}
                </ul>
              </div>
            {/if}
          {:else}
            <p class="text-xs text-slate-400 mt-2 flex items-center gap-1.5">
              <span class="inline-block size-2 rounded-full bg-emerald-500 animate-pulse"></span>
              Running now — this page updates when it finishes.
            </p>
          {/if}
        {/if}
        {#if autopilotError}
          <p class="text-xs text-red-600 mt-2">{autopilotError}</p>
        {/if}
      </div>
    {:else}
      <p class="text-sm text-slate-400">Loading...</p>
    {/if}
  </Card>


<!-- Zen API key -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-2">
      {@html iconSvg('zap', 20)} Zen API key
    </h3>
    {#if !zenKeySet}
    <p class="text-sm text-amber-600 dark:text-amber-400 mb-4">No Zen key — matches will arrive unscored until you add one below.</p>
    {/if}
    {#if zenKeySet}
      <p class="text-xs text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5 mb-3">
        {@html iconSvg('check-circle', 14)}
        Key saved — scoring is active.
      </p>
    {/if}
    <div class="flex gap-2">
      <input
        type="password"
        bind:value={zenKeyValue}
        placeholder={zenKeySet ? 'Replace saved key…' : 'oc_…'}
        autocomplete="off"
        aria-label="Zen API key"
        maxlength="128"
        class="flex-1 min-w-0 px-3 py-2 text-sm bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-lg focus:outline-none focus:border-slate-400"
      />
      <button
        class="px-3 py-2 text-xs font-medium bg-slate-800 text-white rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50"
        disabled={zenKeySaving}
        onclick={saveZenKey}
      >{zenKeySaving ? 'Saving…' : zenKeySaved ? 'Saved' : 'Save'}</button>
    </div>
    {#if zenKeyError}
      <p class="text-xs text-red-600 mt-2">{zenKeyError}</p>
    {/if}
    <div class="mt-4 pt-4 border-t border-slate-100 dark:border-slate-600">
      <label class="wp-label" for="zen-model">Curation model</label>
      <div class="flex gap-2">
        <select
          id="zen-model"
          bind:value={zenModel}
          class="flex-1 min-w-0 px-3 py-2 text-sm bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-lg focus:outline-none focus:border-slate-400"
        >
          <option value="">Default (x-preview-f-free)</option>
          {#each zenModels as m}
            <option value={m}>{m}</option>
          {/each}
          {#if zenModel && !zenModels.includes(zenModel)}
            <option value={zenModel}>{zenModel}</option>
          {/if}
        </select>
        <button
          class="px-3 py-2 text-xs font-medium bg-slate-800 text-white rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50"
          disabled={zenModelSaving || !zenKeySet}
          title={zenKeySet ? '' : 'Save a Zen key first'}
          onclick={saveZenModel}
        >{zenModelSaving ? 'Saving…' : zenModelSaved ? 'Saved' : 'Apply'}</button>
      </div>
    </div>
    <p class="text-xs text-slate-400 dark:text-slate-500 mt-3 leading-relaxed">
      Get one at <a href="https://opencode.ai/auth" target="_blank" rel="noopener noreferrer" class="text-blue-600 dark:text-blue-300 underline">opencode.ai/auth</a> → Billing → copy the key (starts with <code class="bg-slate-100 dark:bg-slate-800 px-1 rounded">oc_</code>). Prepaid credits, pay per request.
    </p>
  </Card>


<!-- Exa API key -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 dark:text-slate-200 mb-2">
      {@html iconSvg('search', 20)} Exa API key
    </h3>
    {#if !exaKeySet}
    <p class="text-sm text-slate-400 dark:text-slate-500 mb-4">Optional — suggests new companies to watch. Uses your Zen key.</p>
    {/if}
    {#if exaKeySet}
      <p class="text-xs text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5 mb-3">
        {@html iconSvg('check-circle', 14)}
        Key saved — brief-driven discovery is active.
      </p>
    {/if}
    <div class="flex gap-2">
      <input
        type="password"
        bind:value={exaKeyValue}
        placeholder={exaKeySet ? 'Replace saved key…' : 'exa key'}
        autocomplete="off"
        aria-label="Exa API key"
        maxlength="128"
        class="flex-1 min-w-0 px-3 py-2 text-sm bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-lg focus:outline-none focus:border-slate-400"
      />
      <button
        class="px-3 py-2 text-xs font-medium bg-slate-800 text-white rounded-lg hover:opacity-90 transition-colors cursor-pointer disabled:opacity-50"
        disabled={exaKeySaving}
        onclick={saveExaKey}
      >{exaKeySaving ? 'Saving…' : exaKeySaved ? 'Saved' : 'Save'}</button>
    </div>
    {#if exaKeyError}
      <p class="text-xs text-red-600 mt-2">{exaKeyError}</p>
    {/if}
    <p class="text-xs text-slate-400 dark:text-slate-500 mt-3 leading-relaxed">A discovery run makes ~30–60 paid lookups.</p>
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
