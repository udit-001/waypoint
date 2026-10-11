# Autopilot debug — the diagnosis walk

Autopilot cycled and nothing showed up. Answer with exactly one of the five
answers — **quiet**, **nothing-survived**, **waiting-on-you**, **degraded**,
**skipped** — and the evidence that names it. The runlog is the single source
of truth: every answer is reachable from `waypoint autopilot runs --json`
alone.

## The walk

Five steps, in order. The first step whose evidence names an answer ends the
walk: state the answer, cite the evidence, stop.

### 1. Last run

`waypoint autopilot runs --json` — newest first, read `runs[0]`.

- `runs` empty (`[]` / `null`) → autopilot has not cycled yet. Say so; offer
  `waypoint autopilot run` or enabling the daemon.
- `verdict: ""` (unfinished run) → a cycle is in flight. Say so; re-check after
  it finishes. `$DATA_DIR/autopilot.log` carries the raw output meanwhile.

### 2. Verdict

`runs[0].verdict` is the answer outright in two cases:

- `degraded` → **degraded**. Evidence: `stageErrors` (which stage broke) and
  `perSource` rows with `errored > 0`. The run's coverage is partial.
- `waiting-on-you` → **waiting-on-you**. Evidence: `postingsShortlisted`
  postings sit unreviewed (`waypoint postings list --status shortlisted
  --json` lists them, if the user asks which).

`quiet` and `nothing-survived` name the answer but not the cause — the walk
continues to collect the evidence showing where the postings went.

### 3. Per-source

`runs[0].perSource` is the evidence table: one row per source.

- Every row `skipped: true` → **skipped**. Evidence: each row's `reason` —
  `selection fallback: aggregators only ...` (no Zen key configured),
  `not selected by source selection`, or `disabled by user`.
- Some rows skipped, others not → the skipped rows are a coverage gap, not the
  answer; note them and read the active rows.
- Active rows all `scraped: 0` → the sources returned nothing. Three or more
  consecutive runs like this is the stale-search signal below.

### 4. Errors

`runs[0].postingsErrored` counts per-posting failures — a posting that failed
curation, or its source's fetch dying. A count under the verdict, never a
verdict of its own: a nonzero count alongside `quiet` means coverage was
partial. Cite it in the evidence; the answer stays the step-2 verdict.

### 5. Ledger query

The evidence for the verdict step 2 named:

- `waypoint postings list --json` with `dismissed` rows → the evidence for
  **nothing-survived**: postings were found, curation dismissed them all.
- Nothing dismissed and nothing shortlisted → the evidence for **quiet**:
  sources ran fine, nothing new fit the brief. Leftover `new` rows are the
  cycle's limit deferring work, not missing curation.

## Stale search

Three or more consecutive `quiet` runs in the `runs` history is the
stale-search signal: the brief no longer matches what the sources return.
Report the streak. Widening the search or re-running discovery is the user's
call, not a step in this walk.

If the JSON cannot name an answer, that is a runlog bug — report it rather
than explain it away.
