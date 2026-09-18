<script>
  // The zen curation-model dropdown, shared by Settings and the Get
  // Started wizard (WP-163): friendly names from the curated catalog,
  // API-family badge suffixes, the shipped default (big-pickle) as the
  // blank option, and a saved pick that is no longer in the catalog kept
  // visible so it can't be silently lost. Option logic lives in
  // lib/zenPicker.js (unit-tested); this component renders it.
  import { zenOptions } from '../lib/zenPicker.js';

  let { models = [], value = $bindable(''), id = 'zen-model', disabled = false } = $props();
</script>

<select
  {id}
  bind:value
  {disabled}
  class="flex-1 min-w-0 px-3 py-2 text-sm bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-lg focus:outline-none focus:border-slate-400"
>
  {#each zenOptions(models, value) as o (o.value || 'default')}
    <option value={o.value}>{o.label}</option>
  {/each}
</select>
