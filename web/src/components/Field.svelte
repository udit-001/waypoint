<script>
  const uid = $props.id();

  // required renders the required-flag marker next to the label — the
  // surface tells the reader which inputs gate completion, before they
  // hit a downstream "incomplete" check.
  let { label, forId = uid, required = false, children } = $props();
</script>

<div>
  <label class="wp-label" for={forId}>{label}{#if required}<span class="text-red-500 ml-1" title="Required" aria-hidden="true">*</span>{/if}</label>
  {@render children()}
</div>
