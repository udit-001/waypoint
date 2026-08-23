# Plan 002 — Autopilot toggle on motion tokens

**Commit:** eac6731 lineage · **Severity:** MED · **Effort:** ~10min

## Current code (Settings.svelte ~214–218)
```svelte
<button role="switch" ... class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors cursor-pointer disabled:opacity-50 {autopilotEnabled ? 'bg-slate-800' : 'bg-slate-300'}" onclick={toggleAutopilot}>
  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {autopilotEnabled ? 'translate-x-6' : 'translate-x-1'}" />
</button>
```
Both use Tailwind default timing (150ms, stock ease) — off-token vs app.css `--dur-fast`(120ms)/`--ease-in-out`.

## Change
Track button class += ` duration-[120ms] ease-[var(--ease-in-out)]`
Knob span class += ` duration-[120ms] ease-[var(--ease-in-out)] active:scale-90`

(:active propagates from pressed button to descendant knob → subtle press squash. Reduced-motion handled globally.)

## Scope boundaries
- Only this toggle; do not touch other buttons' hover transitions
- No spring libraries

## Verification
Flip repeatedly: knob travel feels snappy-symmetric; clicking and holding squashes knob slightly; reduced-motion → instant state flip.
