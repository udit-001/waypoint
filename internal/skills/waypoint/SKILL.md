---
name: waypoint
description: Manage job applications with the waypoint CLI. Use when the user mentions job applications, applying to companies, cover letters, interview prep, career summaries, wants to track their job search, or find new job postings from job portals.
---

`waypoint` CLI. Local SQLite. Every interaction follows the **pipeline**: enroll → enrich → generate → save.

## Discovery

When the user wants to find new jobs or see what's new:

1. **Curation interview first** — `read` [grilling](references/grilling.md) and interview the user round-by-round until the **curation frontier** is empty. Never start scraping until you know what "a good job" means for this user — guessing is how low-quality jobs get fetched and curated.
2. Then `read` [scraping](references/scraping.md) and fetch, filtering every result against the settled **curation brief**. Extraction happens inside Step 4 (Promote picks) — the promoted result leaves the scraping flow enriched, ready for generate/save.

Scrape is the primary path, but Exa is a legitimate discovery fallback when scrapers fall short. The scraping reference's Entry condition decides when.

## Pipeline

### Step 1 — Enroll

At conversation start, check state:
```bash
waypoint jobs stats --json && waypoint profile show --json
```

- `total: 0` + empty `name` → fresh install. Ask conversationally, run commands yourself:
  1. "Name and roles you're targeting?" → `read` [data/profile](references/data/profile.md), write a profile document, `profile set --file <doc>`
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
Write a profile document with the missing fields and apply it:
```bash
waypoint profile set --file <path-to-profile-doc>
```
See [data/profile](references/data/profile.md) for the document rules.

**Done when**: job ID resolved, profile complete.

### Notes — shell-safe

The `--notes` field renders as GitHub-flavoured markdown in the web UI. Write **structured markdown**: headings, lists, tables, blockquotes, bold/italic, task lists, inline code.

Sort every note as **shell-safe** or **shell-unsafe**. Shell-safe notes go inline with `--notes "..."`; shell-unsafe notes — anything containing `$`, backticks, quotes, `!`, `\`, or more than one line — must go through a file via `--notes-file`. Shell-unsafe characters are expanded or re-parsed by **both** bash and PowerShell on the command line.

Write the notes file **with your file tool**, not a shell heredoc — `cat << EOF` and `/tmp/` are bash-only and break under Windows PowerShell. Put it in the platform temp dir (`/tmp` on Unix, `TEMP` on Windows), then pass the path:

```
waypoint jobs update 5 --notes-file <path-to-notes-file>
```

The file is read directly — no shell interpretation of its contents.

Shell-safe (inline):
```
waypoint jobs update 5 --notes "Reached out by recruiter"
```

Shell-unsafe (bad inline — `$` is expanded as a variable, not passed literally):
```
waypoint jobs update 5 --notes "Salary: $35-55/hr — great fit"
```

**Done when**: shell-unsafe notes go via `--notes-file` from a file you wrote with your file tool; only shell-safe strings use inline `--notes`.

### Step 3 — Generate

Every generation follows the same **draft**: pull data → pick options → draft → review. `read` the relevant gen-* reference for its options, structures, and done criteria.

1. `waypoint jobs get <id>` — pull company, position, notes, URL
2. `waypoint profile show --json` — pull name, skills, experience, education (entry `description` fields are the depth for resume/SOP/cover-letter material)
3. `read` the gen-* reference for options (tone, style, type, etc.)
4. Pick options from user request; ask if ambiguous
5. Draft following the reference's structure
6. Validate against its done criteria

**Done when**: all items in the reference's done criteria pass.

### Step 4 — Save

Always save generated content as an artifact. Generated content is long and frequently shell-unsafe, so write it to a file **with your file tool** (in the platform temp dir) and pass it via `-f` — avoids shell escaping, links to job, visible in web UI.
```
waypoint artifacts add --skill <id> --title "<title>" -f <path-to-content-file> --job <id>
```

Multi-variant: `--variants-file <path>`. Title from file: `--title-file <path>`.

**Done when**: artifact saved and confirmed.

## After save

Suggest a natural next step:
- Cover letter → "Follow-up email too?"
- Interview prep → "Career summary as well?"
- First artifact → "`waypoint start` to see in web UI"
- User shared new personal details (experience, education, skills, contact) → "I used this in your [artifact]. Save it to your profile for next time?" → `read` [data/profile](references/data/profile.md) and save them via `profile set --file`

## Data sources

- **Exa MCP** → `read` [data/exa-search](references/data/exa-search.md). Discovery fallback (eligibility per the scraping reference's Entry condition) + research/enrichment on tracked jobs via `jobs update --contact` / `--notes-file`. If exa not connected, offer setup — see [data/exa-setup](references/data/exa-setup.md)
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

### Data & profile — `read` the data/* reference

| Ref | Output |
|-----|--------|
| [data/profile](references/data/profile.md) | profile read/write surface: `show --json` · `schema` template · `set --file` patch |
| [data/job-extract](references/data/job-extract.md) | parse job from URL/PDF/text → jobs add |
| [data/exa-search](references/data/exa-search.md) | discovery fallback + company/people/news research on tracked jobs |
| [data/pdf-extract](references/data/pdf-extract.md) | extract text from PDFs (if pdftotext) |

Skill IDs: `email-generator` `cover-letter` `resume-optimizer` `interview-prep` `career-summary` `statement-of-purpose`

View artifacts: `artifacts list` · `artifacts list --job <id>` · `artifacts get <id>`
