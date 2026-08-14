<script>
  import Field from './Field.svelte';

  let { label, value = '', placeholder = '', type = 'text', oncommit, ...rest } = $props();
  let draft = $state(value ?? '');
  let focused = $state(false);

  // Sync from the server without clobbering an in-progress edit.
  $effect(() => {
    if (!focused && (value ?? '') !== draft) draft = value ?? '';
  });

  function commit() {
    const v = draft.trim();
    if (v !== (value ?? '') && oncommit) oncommit(v);
  }
</script>

<Field {label}>
  <input
    class="wp-input w-full"
    {type}
    {placeholder}
    bind:value={draft}
    onfocus={() => (focused = true)}
    onblur={() => {
      focused = false;
      commit();
    }}
    {...rest}
  />
</Field>
