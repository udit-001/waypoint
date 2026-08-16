<p align="center">
  <img src="web/public/icons/icon-192.svg" width="64" height="64" alt="Waypoint logo">
</p>

# Waypoint — your job search, in one place

Track every application and let your AI assistant write the paperwork.
Runs on your computer: no account, nothing uploaded.

## Install

```bash
curl -sfL https://raw.githubusercontent.com/udit-001/waypoint/main/install.sh | sh
waypoint init
waypoint start    # opens http://localhost:8080
```

Have Go? `go install github.com/udit-001/waypoint/cmd/waypoint@latest` works too.
Update later with `waypoint upgrade`.

## Connect your AI

```bash
waypoint skills install --agent pi.dev
```

Works with `pi.dev`, `claude-code`, `codex`, `opencode`. Then just talk to it:
"track a job at Meta, I applied Monday", "prep me for the Amazon interview".

## What it does

- **Track** — jobs with status, salary, contacts, notes. Kanban board, table, charts.
- **Profile** — work history and skills, one-click LinkedIn import, job-search preferences.
- **Find jobs** — search portals, or watch a company's careers page for new postings.
- **AI writing** — six skills: emails, cover letters, interview prep, career summaries, resume keyword checks, grad-school statements. Every version is saved to its job.
- **Resume privacy** — strips your phone number and email before the resume reaches an AI.

## Your data

One folder: `~/.waypoint/`. Back it up and you're done. Works offline.

[CLI reference](docs/cli.md) · [Architecture](docs/architecture.md) · MIT
