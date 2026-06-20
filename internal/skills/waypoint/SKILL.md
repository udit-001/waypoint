---
name: waypoint
description: Job application tracker CLI
---

`waypoint` CLI. Local SQLite.

## First-run

At conversation start:
```bash
waypoint jobs stats --json && waypoint profile show --json
```

- `total: 0` + empty `name` → fresh install. Ask conversational questions, run commands yourself:
  1. "Name and roles you're targeting?" → `profile set --name "..." --title "..." --skills '["..."]'`
  2. "Jobs already tracking?" → `jobs add "..." "..." --status "..."` per job
  3. "See dashboard?" → `start`
- `total: 0` + has name → no jobs yet, ask if they want to add
- Profile incomplete + jobs exist → ask just missing fields

## Before generating

### 0. Add the job (if not tracked)

`read` [job-extract](references/job-extract.md) — parse details from URL, PDF, or text → `jobs add` flags.

### 1. Resolve the job

No job ID? Search:
```bash
waypoint jobs list --search "<company or role>" --json
```
Found → use ID. Multiple → ask user. None → `jobs add`.

### 2. Profile must be complete

`name`, `title`, `skills` must be non-empty. Missing → ask before generating.
```bash
waypoint profile set --name "Jane Doe" --title "Senior Engineer" --skills '["Go","React","AWS"]'
```

Job resolved + profile complete → `read` skill reference and generate.

### 3. After saving

- Cover letter → "Follow-up email too?"
- Interview prep → "Career summary as well?"
- First artifact → "`waypoint start` to see in web UI"

## External data

- **Exa MCP** → `read` [exa-search](references/exa-search.md) for company/people intel. Save via `jobs update --contact` / `--notes`
- **PDFs** → `read` [pdf-extract](references/pdf-extract.md) if `pdftotext` available

## Commands

| Cmd | Flags |
|-----|-------|
| `jobs add <co> <pos>` | `--status` `--category` `--salary` `--location` `--contact` `--url` `--notes` `--date` `--applied-date` `--reminder` |
| `jobs list` | `--status` `--category` `--search` `--limit` `--all` |
| `jobs get <id>` | `--history` |
| `jobs update <id>` | same as `add` |
| `jobs delete <id>` | `--force` |
| `jobs stats` | |
| `artifacts add` | `--skill` `--title` `--title-file` `-f` `--variant-content` `--variant-file` `--variant-label` `--variants` `--variants-file` `--options` `--options-file` `--job` |
| `artifacts list` | `--skill` `--job` `--all` |
| `artifacts get <id>` | |
| `artifacts delete <id>` | `--force` |
| `artifacts archive <id>` | |
| `categories list\|add\|rename\|delete` | |
| `profile show\|set` | `--name` `--email` `--phone` `--title` `--skills` `--experience` `--education` `--industry` `--greeting-style` `--sign-off` |
| `start` | `--port` (8080) |
| `init` | `--force` |

All: `--db <path>`, `--json`.

## References

| Ref | Output |
|-----|--------|
| [email-generator](references/email-generator.md) | 4 email types × 4 tones |
| [cover-letter](references/cover-letter.md) | cover letter in 4 styles |
| [resume-optimizer](references/resume-optimizer.md) | match %, missing keywords, action verbs |
| [interview-prep](references/interview-prep.md) | role Q&A + research checklist |
| [career-summary](references/career-summary.md) | resume summary in 5 styles |
| [statement-of-purpose](references/statement-of-purpose.md) | SOP in 4 tones |
| [job-extract](references/job-extract.md) | parse job from URL/PDF/text → jobs add |
| [exa-search](references/exa-search.md) | company/people research (if exa MCP) |
| [pdf-extract](references/pdf-extract.md) | extract text from PDFs (if pdftotext) |

## Save as artifacts

Always use `-f` — no shell escaping, linked to job, visible in web UI.

```bash
waypoint artifacts add --skill cover-letter --title "Cover for Google" -f /tmp/cover.txt --job 3
waypoint artifacts add --skill email-generator --title "Follow-up" -f /tmp/email.txt --variant-label Casual --job 3
waypoint artifacts add --skill cover-letter --title "Cover" --variants-file /tmp/variants.json --job 3
waypoint artifacts add --skill interview-prep --title-file /tmp/title.txt -f /tmp/prep.md --job 3
```

Skill IDs: `email-generator` `cover-letter` `resume-optimizer` `interview-prep` `career-summary` `statement-of-purpose`

View: `artifacts list` · `artifacts list --job 3` · `artifacts list --skill cover-letter` · `artifacts get 12`

## Quick ref
```
waypoint jobs add "Google" "SWE" --status Applied --date 2026-06-20
waypoint jobs list --search python --category Tech
waypoint jobs update 1 --status Rejected
waypoint artifacts add --skill cover-letter --title "Cover" -f /tmp/cover.txt --job 1
waypoint start --port 8080
```
