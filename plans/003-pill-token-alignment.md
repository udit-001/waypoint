# Plan 003 — Layout pill timing on tokens

**Commit:** eac6731 lineage · **Severity:** LOW · **Effort:** ~5min

## Current code (TopBar.svelte ~131 and ~139)
Both pill buttons carry `transition-colors` with Tailwind defaults.

## Change
Append ` duration-[120ms] ease-[var(--ease-in-out)]` to each pill's class list (after `transition-colors`).

## Scope boundaries
- No sliding-thumb indicator (two fixed text options — color swap is enough)
- Nothing else in TopBar

## Verification
Toggle pills: background/text swap feels identical in rhythm to the autopilot toggle.
