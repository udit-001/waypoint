<script>
  let { label, children, ...rest } = $props();
</script>

<div {...rest}>
  <label class="wp-label">{label}</label>
  {@render children()}
</div>
