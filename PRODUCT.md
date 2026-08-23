# Waypoint

## Register

Product. A single-user tool (design SERVES the job search) with a dashboard shell: icon rail nav, dense lists, cards, forms. No marketing surfaces.

## Users

Two anchors:

1. **The owner** — technical, dogfoods daily, drives the CLI and agent workflows. Tolerates density, hates noise.
2. **The non-technical friend** — the success metric. Short attention span, ADHD-audited flows, never opens a terminal. Reads Matches for ~2 minutes at a time.

Shared trait: time-poor. Every screen competes with "I should be applying instead."

## Product purpose

Autopilot watches companies and job boards on a schedule, judges each posting against the user's brief (LLM curation), and lands scored matches in **Matches** for a Keep/Skip review. Kept postings promote to **Applications**, the board the user already lives in.

## Aha moment

A scored match appears in Matches without the user doing anything — and one click files it on Applications.

## Personality

Calm, honest, low-stimulus. "On never lies." One decision per glance; depth available via expansion but never forced. Empty states are doorways, not dead ends. Wins register quietly.

## Vocabulary (load-bearing — one name per object)

- **Autopilot** — the scheduled loop
- **Companies** — what it watches (boards.toml)
- **Matches** — the review queue of judged postings
- **Applications** — pursuits (the jobs table)
- UI says **Keep/Skip** ("Add to applications" / "Dismiss"); CLI/API say promote/dismiss (agent-facing, per spec)
- **Brief** — the preferences that drive judgment (Profile → Job Search Preferences)

## Anti-references

- Enterprise dashboard clutter: stat-card walls, badge soup, alert fatigue
- Motivational-slop copy ("You've got this! 🚀")
- Per-posting notification spam (one ping per cycle, counts + top match)

## Design principles

1. Attention is the scarcest resource — never spend it on ceremony.
2. The queue is the source of truth; notifications are nudges.
3. Failures are logged where they're visible (run ledger), never silent, never blocking.
4. Copy states what happened and what to do next; blame the field, not the user.
