<script>
  import Field from './Field.svelte';

  let { label, value = '', options = [], onchange, ...rest } = $props();
</script>

<Field {label}>
  <select class="wp-select" value={value} onchange={(e) => onchange?.(e.currentTarget.value)} {...rest}>
    {#each options as opt}
      <option value={opt.value}>{opt.label}</option>
    {/each}
  </select>
</Field>
