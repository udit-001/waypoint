<script>
  const uid = $props.id();

  let { label, forId = uid, children } = $props();
</script>

<div>
  <label class="wp-label" for={forId}>{label}</label>
  {@render children()}
</div>
