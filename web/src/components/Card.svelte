<script>
  let { href, padding = 'p-5', hover = true, class: className = '', children, onclick, ...rest } = $props();
</script>

{#if href}
  <a {href} class="block bg-white rounded-xl shadow-sm {hover ? 'hover-safe:shadow-md active:scale-[0.96]' : ''} transition-[transform,box-shadow] {padding} {className}" {...rest}>
    {@render children()}
  </a>
{:else}
  <div
    class="bg-white rounded-xl shadow-sm {hover ? 'hover-safe:shadow-md active:scale-[0.96] cursor-pointer' : ''} transition-[transform,box-shadow] {padding} {className}"
    {onclick}
    role={onclick ? 'button' : undefined}
    tabindex={onclick ? '0' : undefined}
    {...rest}
  >
    {@render children()}
  </div>
{/if}
