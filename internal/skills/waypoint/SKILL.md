---
name: waypoint
description: Manage job applications with the waypoint CLI. Use when the user mentions job applications, applying to companies, cover letters, interview prep, career summaries, wants to track their job search, or find new job postings from job portals.
---

`waypoint` CLI. Local SQLite. Every interaction follows the **pipeline**: enroll → enrich → generate → save.

## Discovery

When the user wants to find new jobs or see what's new — `read` [scraping](references/scraping.md). Extraction happens inside Step 4 (Promote picks) — the promoted result leaves the scraping flow enriched, ready for generate/save.

Scrape is the primary path. Do not use Exa for job discovery — the scraping reference instructs when to fall back to Exa if no relevant scrapers exist or all return empty.

## Pipeline

### Step 1 — Enroll

At conversation start, check state:
```bash
waypoint jobs stats --json && waypoint profile show --json
```

- `total: 0` + empty `name` → fresh install. Ask conversationally, run commands yourself:
  1. "Name and roles you're targeting?" → `profile set --name "..." --title "..." --skills '["..."]'`
  2. "Jobs already tracking?" → `jobs add "..." "..." --status "..."` per job
  3. "See dashboard?" → `start`
- `total: 0` + has name → no jobs yet, ask if they want to add
- Profile incomplete + jobs exist → ask just missing fields

**Done when**: profile `name`, `title`, `skills` all non-empty.

### Step 2 — Enrich

Before generating any content, the job must be resolved and the profile complete. No shortcuts.

No job ID? Search:
```bash
waypoint jobs list --search "<company or role>" --json
```
Found → use ID. Multiple → ask user. None → `read` [data/job-extract](references/data/job-extract.md) to parse from URL, PDF, or text, then `jobs add`.

Profile `name`, `title`, `skills` must be non-empty. Missing → ask before generating.
```bash
waypoint profile set --name "Jane Doe" --title "Senior Engineer" --skills '["Go","React","AWS"]'
```

**Done when**: job ID resolved, profile complete.

### Notes — shell-safe

The `--notes` field renders as GitHub-flavoured markdown in the web UI. Write **structured markdown**: headings, lists, tables, blockquotes, bold/italic, task lists, inline code.

Use `--notes-file` for any content that contains `$`, backticks, quotes, or multi-line text — bash interprets these characters on the command line. Write the notes to a temp file first, then pass the path:

```bash
cat > /tmp/notes.md << 'EOF'
## Interview process
1. ~~Recruiter screen~~
2. **On-site** pending

> Follow up by Jun 25 if no reply.
EOF
waypoint jobs update 5 --notes-file /tmp/notes.md
```

The file is read directly — no shell interpretation of its contents. Use `--notes-file` for all research results, multi-section notes, and any text containing `$`, `"`, `` ` ``, `\`, or `!`.

Use inline `--notes "..."` only for short, simple strings with no shell-significant characters:
```bash
waypoint jobs update 5 --notes "Reached out by recruiter"
```

Bad (inline notes with shell-significant chars — bash will expand `$` and break multi-line strings):
```bash
waypoint jobs update 5 --notes "Salary: $35-55/hr — great fit"
```

**Done when**: notes content is written in structured markdown and passed via `--notes-file` unless it is a short, shell-safe string.

### Step 3 — Generate

Every generation follows the same **draft**: pull data → pick options → draft → review. `read` the relevant gen-* reference for its options, structures, and done criteria.

1. `waypoint jobs get <id>` — pull company, position, notes, URL
2. `waypoint profile show --json` — pull name, skills, experience, education
3. `read` the gen-* reference for options (tone, style, type, etc.)
4. Pick options from user request; ask if ambiguous
5. Draft following the reference's structure
6. Validate against its done criteria

**Done when**: all items in the reference's done criteria pass.

### Step 4 — Save

Always save generated content as an artifact. Use `-f` (file input) — avoids shell escaping, links to job, visible in web UI.
```bash
waypoint artifacts add --skill <id> --title "<title>" -f /tmp/content.txt --job <id>
```

Multi-variant: `--variants-file /tmp/variants.json`. Title from file: `--title-file /tmp/title.txt`.

**Done when**: artifact saved and confirmed.

## After save

Suggest a natural next step:
- Cover letter → "Follow-up email too?"
- Interview prep → "Career summary as well?"
- First artifact → "`waypoint start` to see in web UI"
- User shared new personal details (experience, education, skills, contact) → "I used this in your [artifact]. Save it to your profile for next time?" → `profile set`

## Data sources

- **Exa MCP** → `read` [data/exa-search](references/data/exa-search.md). Save research via `jobs update --contact` / `--notes-file`. If exa not connected, offer setup — see [data/exa-setup](references/data/exa-setup.md). Not for primary job discovery — use the Discovery section for that
- **PDFs** → `read` [data/pdf-extract](references/data/pdf-extract.md). Missing `pdftotext`? Install it — see the reference for each OS.
- **Job parsing** → `read` [data/job-extract](references/data/job-extract.md)

## References

### Generation skills — `read` the gen-* reference for options, structures, and done criteria

| Ref | Output |
|-----|--------|
| [gen-email-generator](references/gen-email-generator.md) | 4 email types × 4 tones |
| [gen-cover-letter](references/gen-cover-letter.md) | cover letter in 4 styles |
| [gen-resume-optimizer](references/gen-resume-optimizer.md) | match %, missing keywords, action verbs |
| [gen-interview-prep](references/gen-interview-prep.md) | role Q&A + research checklist |
| [gen-career-summary](references/gen-career-summary.md) | resume summary in 5 styles |
| [gen-statement-of-purpose](references/gen-statement-of-purpose.md) | SOP in 4 tones |

### Data extraction — `read` the data/* reference

| Ref | Output |
|-----|--------|
| [data/job-extract](references/data/job-extract.md) | parse job from URL/PDF/text → jobs add |
| [data/exa-search](references/data/exa-search.md) | company/people/news research on tracked jobs (not discovery) |
| [data/pdf-extract](references/data/pdf-extract.md) | extract text from PDFs (if pdftotext) |

Skill IDs: `email-generator` `cover-letter` `resume-optimizer` `interview-prep` `career-summary` `statement-of-purpose`

View artifacts: `artifacts list` · `artifacts list --job <id>` · `artifacts get <id>`
