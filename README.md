<p align="center">
  <img src="web/public/icons/icon-192.svg" width="64" height="64" alt="Waypoint logo">
</p>

# Waypoint — your job search, in one place

A private job application tracker that lives on your computer and works with
your AI assistant. Waypoint remembers every application, finds new openings,
and writes the paperwork — cover letters, emails, interview prep — so you
don't have to.

## Why Waypoint

- **Everything in one place.** Every application, note, and document for your
  job search — searchable and organized, not scattered across spreadsheets,
  browser tabs, and your inbox.
- **Private by default.** Your data lives on your computer, in one folder.
  Nothing is uploaded anywhere. No account, no cloud, no tracking.
- **Your AI does the grunt work.** Waypoint pairs with AI assistants like
  Claude, Codex, and pi. You say *"add an application at Google"* or
  *"write a cover letter for the Stripe role"* — the assistant does the rest.

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
`pi.dev`, `claude-code`, `codex`, `opencode`. Now just talk to it naturally —
"track a job at Meta, I applied Monday", "what should I follow up on?",
"prep me for the Amazon interview".

To update Waypoint later: `waypoint upgrade`.

## What you can do

**Track every application**
Add jobs with status, salary, contacts, links, and notes. See your search at
a glance on the dashboard — progress charts, a Kanban board for your
pipeline, and a sortable list. Search everything instantly.

**Tell Waypoint about yourself**
Build a profile once — work history, education, skills — and import it from
your public LinkedIn page with one click. Set your job-search preferences
(role, location, constraints) so the AI knows what to look for and tailor
content to you.

**Find new openings**
Search job portals directly, and keep an eye on specific companies: ask your
assistant to watch a company's careers page and it will flag new postings
for you to review and track.

**Let AI write for you**
Waypoint's built-in skills generate application emails, cover letters,
interview prep, career summaries, and more — in multiple tones and styles,
all grounded in your profile. Every version is saved to the job it belongs
to, so nothing gets lost.

Sharing your resume with an AI? Waypoint can strip your phone number and
email from the copy it hands over automatically.

## Where your data lives

One folder on your computer: `~/.waypoint/`. Back that folder up and you've
backed up everything. Waypoint also works offline — it's a web app that runs
from your own machine, not from the internet.

## The dashboard

- **Dashboard** — charts and stats on how your search is going
- **Applications** — your pipeline as a Kanban board, list, or table
- **Search** — one box across jobs, notes, and documents
- **Profile** — your details, LinkedIn import, and job-search preferences
- **Documents** — everything your AI has written, organized per job
- **Categories** — group applications your way
- **Settings** — appearance and reference

Press `/` anywhere to jump to search.

## Going deeper

- **[CLI reference](docs/cli.md)** — every command, if you like the terminal
- **[Architecture](docs/architecture.md)** — how Waypoint is built, for contributors

## License

MIT
