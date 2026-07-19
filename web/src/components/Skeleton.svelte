<script>
  // WP-95 — shared loading primitive. Three variants (bar / block / circle)
  // cover the loading treatments in Applications.svelte (List rows + Kanban
  // cards). Callers size with `class`; this component owns the shimmer +
  // shape only. The reduced-motion override in app.css collapses the
  // shimmer to a static block via the global animation-duration rule.

  let {
    variant = 'bar',
    class: className = '',
  } = $props();

  // Caller-supplied `rounded-*` should override the default radius; we
  // can't rely on source order between Tailwind utilities (see lific's
  // Skeleton.svelte for the rationale — same Tailwind caveat).
  let hasRoundedOverride = $derived(/\brounded(?:-|\b)/.test(className));
  let defaultRounded = $derived(
    hasRoundedOverride ? '' :
    variant === 'circle' ? 'rounded-full' :
    variant === 'block' ? 'rounded-lg' : 'rounded',
  );
</script>

<div
  class="skeleton-shimmer bg-slate-200/70 dark:bg-slate-700/50 {defaultRounded} {className}"
  aria-hidden="true"
></div>

<style>
  .skeleton-shimmer {
    background-image: linear-gradient(
      100deg,
      transparent 30%,
      var(--shimmer-band, rgba(255, 255, 255, 0.45)) 50%,
      transparent 70%
    );
    background-size: 200% 100%;
    background-repeat: no-repeat;
    animation: skeleton-sweep 1.6s ease-in-out infinite;
  }
  :global([data-theme="dark"]) .skeleton-shimmer {
    --shimmer-band: rgba(255, 255, 255, 0.06);
  }
  @keyframes skeleton-sweep {
    0%   { background-position: 150% 0; }
    100% { background-position: -50% 0; }
  }
</style>
