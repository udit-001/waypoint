<script>
  import { onMount } from 'svelte';

  let { open = false, title = '', size = 'md', onclose } = $props();

  function handleKeydown(e) {
    if (e.key === 'Escape' && open) onclose?.();
  }

  function handleOverlayClick(e) {
    if (e.target === e.currentTarget) onclose?.();
  }

  $effect(() => {
    if (open) {
      document.body.style.overflow = 'hidden';
      document.addEventListener('keydown', handleKeydown);
    } else {
      document.body.style.overflow = '';
      document.removeEventListener('keydown', handleKeydown);
    }
    return () => {
      document.body.style.overflow = '';
      document.removeEventListener('keydown', handleKeydown);
    };
  });
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm"
    onclick={handleOverlayClick}
  >
    <div class="bg-stone-50 rounded-xl border border-slate-200 shadow-lg flex flex-col w-[90%] {size === 'lg' ? 'max-w-3xl' : 'max-w-lg'} max-h-[85vh] animate-in">
      <div class="flex items-center justify-between px-5 py-4 border-b border-slate-200">
        <h3 class="text-base font-semibold text-slate-800">{title}</h3>
        <button
          class="text-slate-400 hover:text-slate-600 text-xl px-1 cursor-pointer"
          onclick={onclose}
        >×</button>
      </div>
      <div class="flex-1 px-5 py-4 overflow-y-auto">
        {@render $props.children()}
      </div>
    </div>
  </div>
{/if}

<style>
  .animate-in {
    animation: modalIn 0.2s ease;
  }
  @keyframes modalIn {
    from { opacity: 0; transform: scale(0.95) translateY(10px); }
    to { opacity: 1; transform: scale(1) translateY(0); }
  }
</style>
