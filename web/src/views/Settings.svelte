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

  onMount(async () => {
    setPage({ title: 'Settings' });

    await api.settings.ensure();
    settingsData = api.settings.value;
    currentFont = document.documentElement.dataset.font || localStorage.getItem('waypoint_font') || 'sans';
  });

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
      <pre bind:this={cliPre} class="bg-slate-50 p-4 rounded-lg text-sm text-slate-600 leading-relaxed overflow-x-auto font-mono">waypoint jobs add "Company" "Position" --status Applied --category Tech
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
