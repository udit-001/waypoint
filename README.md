<p align="center">
  <img src="web/public/icons/icon-192.svg" width="64" height="64" alt="Waypoint logo">
</p>

# Waypoint — your job search, in one place

A private job application tracker that runs on your computer and pairs with
your AI assistant. Waypoint remembers every application, finds new openings,
and writes the paperwork: cover letters, emails, interview prep.

## Why Waypoint

- **Everything in one place.** Applications, notes, and documents, all
  searchable — not spread across spreadsheets and browser tabs.
- **Private by default.** Your data lives in one folder on your computer.
  There's no account, and nothing gets uploaded.
- **Your AI does the grunt work.** Waypoint works with the assistant you
  already use. You ask in plain language; it runs the commands.

## Get started

**1. Install Waypoint**

No prerequisites — paste this into a terminal:

```bash
curl -sfL https://raw.githubusercontent.com/udit-001/waypoint/main/install.sh | sh
```

(If you already have Go installed, `go install github.com/udit-001/waypoint/cmd/waypoint@latest` works too.)

**2. Open the dashboard**

```bash
waypoint init
waypoint start
```

Your browser opens at `http://localhost:8080`.

**3. Connect your AI assistant**

```bash
waypoint skills install --agent pi.dev
```

That teaches your assistant how to run Waypoint for you. Supported agents:
`pi.dev`, `claude-code`, `codex`, `opencode`. From here you can just talk:
"track a job at Meta, I applied Monday", "what should I follow up on?",
"prep me for the Amazon interview".

To update Waypoint later: `waypoint upgrade`.

## What you can do

**Track every application**
Add jobs with status, salary, contacts, links, and notes. The dashboard shows
where things stand: charts, a Kanban board for your pipeline, a sortable
table when you want rows.

**Tell Waypoint about yourself**
Build a profile once: work history, education, skills. If your LinkedIn page
is public, import it with one click. Then set your job-search preferences —
target role, locations, salary floor — and the assistant writes with those
in mind.

**Find new openings**
Search job portals directly, and keep an eye on specific companies: ask your
assistant to watch a company's careers page and it will flag new postings
for you to review and track.

**Let AI write for you**
Six built-in skills: application emails, cover letters, interview prep,
career summaries, resume keyword checks, and statements of purpose for grad
school. You pick the tone; every version is saved to the job it belongs to.

When you hand your resume to an AI, Waypoint can strip your phone number
and email from the copy first.

## Where your data lives

One folder: `~/.waypoint/`. Back that folder up and you've backed up
everything. The dashboard runs from your own machine, so it works offline.

## The dashboard

- **Dashboard** — charts and stats on how your search is going
- **Applications** — Kanban board, list, or table
- **Search** — one box across jobs, notes, and documents
- **Profile** — your details, LinkedIn import, job-search preferences
- **Documents** — what your AI has written, organized per job
- **Categories** — group applications your way
- **Settings** — appearance and reference

Press `/` anywhere to jump to search.

## Going deeper

- **[CLI reference](docs/cli.md)** — every command, if you like the terminal
- **[Architecture](docs/architecture.md)** — how Waypoint is built, for contributors

## License

MIT
