<script>
import { setPage } from '../stores/page.svelte.js';
  import { iconSvg } from '../lib/icons.js';
  import { onMount } from 'svelte';
  import Spinner from '../components/Spinner.svelte';
  import Card from '../components/Card.svelte';
  import * as api from '../stores/api.svelte.js';

  let settingsData = $state(null);
  let currentFont = $state('sans');
  let cliPre = $state(null);
  let copiedCli = $state(false);
  let autopilotEnabled = $state(null);
  let autopilotCadence = $state(6);
  let lastRun = $state(null);
  let autopilotError = $state(null);

  onMount(async () => {
    setPage({ title: 'Settings' });

    await api.settings.ensure();
    settingsData = api.settings.value;
    currentFont = document.documentElement.dataset.font || localStorage.getItem('waypoint_font') || 'sans';

    // Load autopilot data.
    try {
      const res = await fetch('/api/autopilot');
      if (res.ok) {
        const data = await res.json();
        autopilotEnabled = data.enabled;
        autopilotCadence = data.cadence || 6;
        lastRun = data.lastRun;
      }
    } catch {}
  });

  async function toggleAutopilot() {
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
    }
  }

  function setFont(font) {
    currentFont = font;
    document.documentElement.dataset.font = font;
    localStorage.setItem('waypoint_font', font);
  }

  async function copyCli() {
    if (!cliPre) return;
    await navigator.clipboard.writeText(cliPre.textContent);
    copiedCli = true;
    setTimeout(() => copiedCli = false, 1500);
  }
</script>

<div class="space-y-4">
  <!-- App Settings -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-2">
      {@html iconSvg('sliders', 20)} App Settings
    </h3>
    <p class="text-sm text-slate-400 mb-6">Settings are managed via the CLI.</p>
    {#if settingsData}
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Default View</label>
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
          <label class="block text-xs font-medium uppercase tracking-wide text-slate-400 mb-1">Items Per Page</label>
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
      <span class="text-lg">T</span> Typography
    </h3>
    <p class="text-sm text-slate-400 mb-6">Choose your preferred reading font.</p>
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

  <!-- Autopilot -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-2">
      {@html iconSvg('zap', 20)} Autopilot
    </h3>
    <p class="text-sm text-slate-400 mb-4">Run your job search automatically in the background.</p>
    {#if autopilotEnabled !== null}
      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <span class="text-sm text-slate-700">Enabled</span>
          <button
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors cursor-pointer {autopilotEnabled ? 'bg-slate-800' : 'bg-slate-300'}"
            onclick={toggleAutopilot}
          >
            <span
              class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {autopilotEnabled ? 'translate-x-6' : 'translate-x-1'}"
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
              {new Date(lastRun.startedAt).toLocaleString()}
            </span>
          </div>
        {/if}
        {#if autopilotError}
          <p class="text-xs text-red-600 mt-2">{autopilotError}</p>
        {/if}
      </div>
    {:else}
      <p class="text-sm text-slate-400">Loading...</p>
    {/if}
  </Card>

  <!-- CLI Reference -->
  <Card hover={false}>
    <h3 class="flex items-center gap-2 text-base font-semibold text-slate-800 mb-3">
      <span class="text-lg">{@html iconSvg('copy', 20)}</span> CLI Quick Reference
    </h3>
    <div class="relative">
      <button
        class="absolute top-2 right-2 px-2.5 py-1 rounded text-xs font-medium cursor-pointer transition-colors {copiedCli ? 'bg-emerald-100 text-emerald-700' : 'bg-white text-slate-600 hover:bg-slate-100 border border-slate-200'}"
        onclick={copyCli}
      >{copiedCli ? '✓ Copied' : 'Copy'}</button>
      <pre bind:this={cliPre} class="bg-slate-50 p-4 pr-20 rounded-lg text-sm text-slate-600 leading-relaxed overflow-x-auto font-mono">waypoint jobs add "Company" "Position" --status Applied --category Tech
waypoint jobs list --status Applied
waypoint jobs update 42 --status Offer --notes "Got the offer!"
waypoint jobs delete 42
waypoint jobs stats
waypoint jobs get 42 --history
waypoint profile show
waypoint categories list</pre>
    </div>
  </Card>
</div>
