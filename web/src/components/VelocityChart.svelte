<script>
  // WP-97 — velocity sparkline (Tufte-faithful).
  // One chart: applications per week over the last 8 weeks.
  //
  // Tufte principles applied (from /tufte analysis + the spec):
  //   - Range frame: y-axis spans only the data's actual range
  //   - One focal accent: most recent week (red, larger, labeled)
  //   - Delta annotation in report voice ("up from N" / "down from N")
  //   - No gridlines except a faint 0-baseline
  //   - Endpoints only (first = muted anchor, last = focal) — no
  //     markers on intermediate points
  //   - Sorted ascending L->R, no legend, no border, no shadows

  import { weeklyCounts, weeklySummary } from '../lib/weekly.js';

  let { jobs = [], numWeeks = 8 } = $props();

  let weeks = $derived(weeklyCounts(jobs, new Date(), numWeeks));
  let summary = $derived(weeklySummary(weeks));

  // Chart geometry — matches prototype/jobs-view-merged.html.
  const VB_W = 560, VB_H = 110;
  const PAD_L = 40;
  const PLOT_RIGHT = 500;
  const PLOT_TOP = 20, PLOT_BOTTOM = 85;
  const PLOT_W = PLOT_RIGHT - PAD_L;
  const xStep = PLOT_W / Math.max(1, numWeeks - 1);

  let realMax = $derived(Math.max(0, ...weeks.map(w => w.count)));
  let maxCount = $derived(Math.max(1, realMax)); // guard against div-by-zero

  function xFor(i) { return PAD_L + i * xStep; }
  function yFor(count) {
    return PLOT_BOTTOM - (count / maxCount) * (PLOT_BOTTOM - PLOT_TOP);
  }

  let points = $derived(weeks.map((w, i) => ({ ...w, x: xFor(i), y: yFor(w.count) })));
  let polylinePts = $derived(points.map(p => `${p.x},${p.y}`).join(' '));

  let focal = $derived(points[points.length - 1]);
  let prev = $derived(points[points.length - 2]);
  let delta = $derived(focal.count - prev.count);
  let deltaLabel = $derived(
    delta > 0 ? `↑ from ${prev.count}`
    : delta < 0 ? `↓ from ${prev.count}`
    : 'same'
  );

  let avgLabel = $derived(
    summary.avg === 0 ? '0'
    : Number.isInteger(summary.avg) ? String(summary.avg)
    : summary.avg.toFixed(1)
  );
</script>

<div class="px-6 py-3 bg-slate-50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700">
  <div class="max-w-5xl mx-auto">
    <div class="flex items-baseline justify-between mb-1">
      <span class="text-[10px] uppercase tracking-wider text-slate-400 dark:text-slate-500 font-semibold">
        Applications per week · {numWeeks}-week trend
      </span>
      <span class="text-[10px] text-slate-400 dark:text-slate-500 tabular-nums">
        {summary.total} total · {avgLabel} avg/week
      </span>
    </div>
    <svg viewBox="0 0 {VB_W} {VB_H}" width="100%" style="overflow: visible; max-width: 640px; display: block;" role="img" aria-label="Applications per week over {numWeeks} weeks">
      <!-- Range frame: y-axis only spans the data's actual range.
           Colors use CSS vars so they adapt to dark mode automatically. -->
      <line x1={PAD_L} y1={PLOT_TOP} x2={PAD_L} y2={PLOT_BOTTOM} style="stroke: var(--color-slate-400)" stroke-width="0.5"/>
      <text x={PAD_L - 5} y={PLOT_TOP + 4} text-anchor="end" font-family="ui-monospace, monospace" font-size="10" style="fill: var(--color-slate-400)">{realMax}</text>
      <text x={PAD_L - 5} y={PLOT_BOTTOM + 4} text-anchor="end" font-family="ui-monospace, monospace" font-size="10" style="fill: var(--color-slate-400)">0</text>

      <!-- Faint 0-baseline only (no gridlines at every unit) -->
      <line x1={PAD_L} y1={PLOT_BOTTOM} x2={PLOT_RIGHT} y2={PLOT_BOTTOM} style="stroke: var(--color-slate-200)" stroke-width="0.5"/>

      <!-- The line: 1.5px, theme-aware via --color-slate-700 -->
      <polyline fill="none" style="stroke: var(--color-slate-700)" stroke-width="1.5" points={polylinePts}/>

      <!-- First point: muted, small (historical anchor) -->
      <circle cx={points[0].x} cy={points[0].y} r="2" style="fill: var(--color-slate-400)"/>
      <text x={points[0].x} y={VB_H - 7} text-anchor="middle" font-family="ui-monospace, monospace" font-size="9" style="fill: var(--color-slate-400)">{points[0].label}</text>

      <!-- Intermediate week labels (muted, no markers) -->
      {#each points.slice(1, -1) as p}
        <text x={p.x} y={VB_H - 7} text-anchor="middle" font-family="ui-monospace, monospace" font-size="9" style="fill: var(--color-slate-400)">{p.label}</text>
      {/each}

      <!-- Most recent point: focal accent (red, larger, labeled) -->
      <circle cx={focal.x} cy={focal.y} r="3.5" fill="#bf616a"/>
      <text x={focal.x} y={focal.y - 11} text-anchor="middle" font-family="ui-monospace, monospace" font-size="12" fill="#bf616a" font-weight="700">{focal.count}</text>
      <text x={focal.x} y={VB_H - 7} text-anchor="middle" font-family="ui-monospace, monospace" font-size="9" fill="#bf616a" font-weight="600">now</text>

      <!-- Delta annotation (number first, then meaning — Tufte report voice) -->
      <text x={focal.x + 10} y={focal.y} text-anchor="start" font-family="ui-monospace, monospace" font-size="10" fill="#bf616a" dominant-baseline="middle">{deltaLabel}</text>
    </svg>
  </div>
</div>
