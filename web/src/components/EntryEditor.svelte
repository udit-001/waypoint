<script>
  import { iconSvg } from '../lib/icons.js';

  // Structured list editor for experience/education. Entries are objects with
  // primary/secondary text fields plus partial ISO start/end dates (YYYY-MM);
  // an empty end means "present" (the Current checkbox).
  let {
    entries = [],
    primaryKey,
    primaryLabel,
    primaryPlaceholder,
    secondaryKey,
    secondaryLabel,
    secondaryPlaceholder,
    descriptionKey = 'description',
    descriptionPlaceholder = 'Details, achievements, impact…',
    readonly = false,
    onchange,
  } = $props();

  let rows = $state([]);

  function sync() {
    rows = (entries ?? []).map((e) => ({
      primary: e[primaryKey] || '',
      secondary: e[secondaryKey] || '',
      description: e[descriptionKey] || '',
      start: e.start || '',
      end: e.end || '',
      current: (e.end || '') === '',
    }));
  }
  sync();
  $effect(sync);

  function toEntries() {
    return rows
      .map((r) => ({
        [primaryKey]: r.primary.trim(),
        [secondaryKey]: r.secondary.trim(),
        [descriptionKey]: r.description.trim(),
        start: r.start,
        end: r.current ? '' : r.end,
      }))
      .filter(
        (e) =>
          e[primaryKey] !== '' ||
          e[secondaryKey] !== '' ||
          e[descriptionKey] !== '' ||
          e.start !== '' ||
          e.end !== '',
      );
  }

  function commit() {
    onchange?.(toEntries());
  }

  function addRow() {
    rows = [...rows, { primary: '', secondary: '', description: '', start: '', end: '', current: false }];
  }

  function removeRow(i) {
    rows = rows.filter((_, idx) => idx !== i);
    commit();
  }

  function toggleCurrent(i) {
    rows[i].current = !rows[i].current;
    if (rows[i].current) {
      rows[i].end = '';
    } else if (rows[i].end === '') {
      // Unchecking "Current" without an end date defaults to the current month.
      const now = new Date();
      rows[i].end = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
    }
    commit();
  }
</script>

<div class="space-y-3">
  {#if readonly}
    <!-- Read-only (WP-117): LinkedIn-style rows — title bold, company + dates
         muted on one line, description as a bullet list. -->
    <div class="space-y-4">
      {#each rows as row}
        {#if row.primary || row.secondary || row.start || row.end || row.description}
          <div>
            <div class="text-sm font-semibold text-slate-800 dark:text-slate-200">{row.primary}</div>
            {#if row.secondary || row.start || row.end}
              <div class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                {row.secondary}{#if row.secondary && (row.start || row.end)}{' · '}{/if}{#if row.start}{row.start}{/if}{#if row.end}{' – '}{row.end}{:else if row.start}{' – present'}{/if}
              </div>
            {/if}
            {#if row.description}
              <ul class="mt-1 space-y-0.5 text-sm text-slate-600 dark:text-slate-300">
                {#each row.description.split('\n').filter((l) => l.trim()) as line}
                  <li class="flex gap-1.5"><span class="text-slate-400 select-none">•</span><span>{line}</span></li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}
      {/each}
    </div>
  {:else}
  {#each rows as row, i}
    <div class="rounded-lg border border-slate-200 dark:border-slate-600 p-3 space-y-2">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <div>
          <label class="wp-label">{primaryLabel}</label>
          <input
            class="wp-input w-full"
            placeholder={primaryPlaceholder}
            bind:value={rows[i].primary}
            onblur={commit}
          />
        </div>
        <div>
          <label class="wp-label">{secondaryLabel}</label>
          <input
            class="wp-input w-full"
            placeholder={secondaryPlaceholder}
            bind:value={rows[i].secondary}
            onblur={commit}
          />
        </div>
      </div>
      <div class="flex items-end gap-2">
        <div class="flex-1">
          <label class="wp-label">Start</label>
          <input
            class="wp-input w-full"
            type="month"
            placeholder="YYYY-MM"
            value={row.start}
            onchange={(e) => {
              rows[i].start = e.currentTarget.value;
              commit();
            }}
          />
        </div>
        <div class="flex-1">
          <label class="wp-label">End</label>
          <input
            class="wp-input w-full"
            type="month"
            placeholder="YYYY-MM"
            value={row.end}
            disabled={row.current}
            onchange={(e) => {
              rows[i].end = e.currentTarget.value;
              commit();
            }}
          />
        </div>
        <label class="flex items-center gap-1.5 pb-2.5 text-xs text-slate-500 dark:text-slate-400 cursor-pointer select-none">
          <input type="checkbox" checked={row.current} onchange={() => toggleCurrent(i)} class="accent-slate-700 dark:accent-slate-400 size-3.5" />
          Current
        </label>
        <button
          type="button"
          class="size-7 grid place-items-center rounded-md text-slate-400 hover:text-red-500 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors cursor-pointer bg-transparent border-none shrink-0"
          aria-label="Remove entry"
          onclick={() => removeRow(i)}
        >{@html iconSvg('close', 15)}</button>
      </div>
      <div>
        <label class="wp-label">Description</label>
        <textarea
          class="wp-input w-full resize-y min-h-16"
          rows="2"
          placeholder={descriptionPlaceholder}
          value={row.description}
          oninput={(e) => {
            rows[i].description = e.currentTarget.value;
          }}
          onblur={commit}
        />
      </div>
    </div>
  {/each}
  <button
    type="button"
    class="inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-colors cursor-pointer bg-transparent border-none p-0"
    onclick={addRow}
  >{@html iconSvg('plus', 14)} Add entry</button>
  {/if}
</div>
