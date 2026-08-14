<script>
  import { iconSvg } from '../lib/icons.js';

  let { value = [], placeholder = 'Type and press Enter', prettify = (v) => v, onchange } = $props();

  // The brief returns null (not []) for empty list preferences — coerce so the
  // length/some calls and the each block never see null.
  const items = $derived(value ?? []);

  let draft = $state('');
  let inputEl = $state(null);

  function addChip() {
    const trimmed = draft.trim();
    if (!trimmed) return;
    if (items.some((v) => v.toLowerCase() === trimmed.toLowerCase())) {
      draft = '';
      return;
    }
    onchange([...items, trimmed]);
    draft = '';
    inputEl?.focus();
  }

  function removeChip(i) {
    const next = items.slice();
    next.splice(i, 1);
    onchange(next);
  }

  function onKeydown(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      addChip();
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      removeChip(items.length - 1);
    }
  }
</script>

<div
  class="flex flex-wrap items-center gap-1.5 rounded-lg border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-800 px-2 py-1.5 min-h-[38px] transition-colors focus-within:border-slate-400 dark:focus-within:border-slate-400"
>
  {#each items as v, i}
    <span
      class="inline-flex items-center gap-1 bg-slate-100 dark:bg-slate-700 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-200 rounded-full pl-2.5 pr-1 py-0.5 text-xs"
    >
      {prettify(v)}
      <button
        type="button"
        class="size-4 inline-flex items-center justify-center rounded-full text-slate-400 hover:text-slate-700 dark:hover:text-slate-100 hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors cursor-pointer bg-transparent border-none"
        aria-label="Remove {v}"
        onclick={() => removeChip(i)}
      >{@html iconSvg('close', 12)}</button>
    </span>
  {/each}
  <input
    bind:this={inputEl}
    bind:value={draft}
    onkeydown={onKeydown}
    onblur={addChip}
    class="flex-1 min-w-32 outline-none text-sm text-slate-700 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-500 bg-transparent"
    placeholder={items.length === 0 ? placeholder : 'Add another…'}
  />
</div>
