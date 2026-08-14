# Grilling — curation interview before discovery

Purpose: never fetch and curate jobs before the user has settled what "a good
job" means for them. Turn that into a **curation brief** that the scraping flow
can encode and filter against.

## When

At the start of any job **discovery** request (a scrape, "find me jobs",
"what's new"). Run this interview **before** calling any scraper or Exa search.

## What a curation brief is

A settled set of curation decisions. It drives the whole discovery: how to
pick scrapers, which flags `scrape run` gets, and which results get promoted or
dismissed. A decision not yet answered by the user is not part of the brief.

## How — the design tree

Map the curation decisions as a **design tree**: every decision branches into
the decisions that hang off it. Work the tree in **rounds**.

The **frontier** is every decision whose prerequisites are already settled —
the questions you can ask _now_. Settled meaning: answered by the user in a
prior round, or already present in the profile. Ask the whole frontier in one
round: number each question and give your recommended answer. Then **stop and
wait** for the user's answers before the next round.

Format each question like this:

```
❓ **Q1** - **<title>**: <question, possibly multiple paragraphs, with choices>

➡️ <your recommended answer>
```

Each round reshapes the tree: settled decisions push the frontier outward and
unblock questions that depended on them. Recompute the frontier and ask the
next round. A question whose answer depends on another still open in this round
belongs to a **later** round.

## The curation decision tree

Seed the first round from the frontier — the items below not already answered
by `profile brief --json` (the curation-brief readout) or by the user's stated
intent. Each item names the waypoint mechanism it feeds.

| Decision | What it controls in waypoint |
|----------|------------------------------|
| **Roles & titles** — exact target titles and variations | `profile.title`; the `-q/--query` keyword on `scrape run` |
| **Seniority / level** — junior / mid / senior / staff, YoE window | sets the expectation when judging results at promote/dismiss |
| **Location & remote** — onsite cities, remote-friendly, relocation | `-l/--location` and `--remote` (remote/hybrid/onsite; LinkedIn only) |
| **Companies** — targets and avoids, size range | promotes/dismisses — not a server-side filter |
| **Industry / domain** — sectors wanted or avoided | matching `profile.industry` against each scraper's `categories` picks which scrapers run |
| **Keywords & dealbreakers** — must-appear vs must-not-appear | must-appears feed `-q/--query`; dealbreakers are judged at promote/dismiss (no exclude flag exists) |
| **Compensation** — floor / range | comparing `metadata.salary` when present; dropping below-floor results at promote |
| **Visa / sponsorship** — required or not | judged per result at promote/dismiss |
| **Recency** — how fresh: today / week / month | `--jobage <days>` plus `--today <YYYY-MM-DD>`; read back as `meta.today` — the anchor results are judged against |
| **Volume** — how many to surface, shortlist bar | `--limit <n>`; the must-have bar at promote |

## Grounding

Finding **facts** is your job, never the user's. `profile brief --json` groups
what's already stored — facts, constraints, preferences — and lists the still
open frontier; pull a stored fact (title, skills, structured experience,
education, industry) from it or from the user's tracked jobs, and don't ask.
**Decisions** are the user's: put each genuinely open one to them and wait. A
fact is asked for only when it is empty **and** unseeded (no resume / public
LinkedIn via Exa), and then once.

Persist every answered decision as you settle it: `profile set --<flag> "<value>"`.
The web profile is the human fallback; the brief readout is the agent's single
source of truth.

## After setup — one confirmation line

Interview only the open items, once, at first setup. On later sessions the
brief is already stored: read it with `profile brief --json`, skip the
interview, and show a single preference-only confirmation line inviting a
correction in the web profile, e.g.:

```
Searching: hybrid, Bengaluru/Delhi, targets Gojek & Flipkart. Change in the web profile if wrong.
```

## Done when

The frontier is empty — every curation decision the profile didn't already
answer has been answered, nothing silently assumed. Lock the results into the
**curation brief**, then encode it into `scrape run` flags and carry it through
the promote/dismiss steps of [scraping](scraping.md). Do not fetch until the
user confirms the brief matches their intent.