<script>
  // The zen curation-model dropdown, shared by Settings and the Get
  // Started wizard (WP-163): friendly names from the curated catalog,
  // API-family badge suffixes, the shipped default (big-pickle) as the
  // blank option, and a saved pick that is no longer in the catalog kept
  // visible so it can't be silently lost.
  let { models = [], value = $bindable(''), id = 'zen-model', disabled = false } = $props();

  const familyLabel = {
    'openai-completions': 'chat',
    'openai-responses': 'responses',
    'anthropic-messages': 'anthropic',
  };
</script>

<select
  {id}
  bind:value
  {disabled}
  class="flex-1 min-w-0 px-3 py-2 text-sm bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-100 border border-slate-200 dark:border-slate-600 rounded-lg focus:outline-none focus:border-slate-400"
>
  <option value="">Default (big-pickle)</option>
  {#each models as m (m.id)}
    <option value={m.id}>{m.name || m.id}{m.api ? ` · ${familyLabel[m.api] ?? m.api}` : ''}</option>
  {/each}
  {#if value && !models.some((x) => x.id === value)}
    <option value={value}>{value}</option>
  {/if}
</select>
