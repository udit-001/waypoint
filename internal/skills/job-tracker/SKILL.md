---
name: job-tracker
description: Manage job applications using the job-tracker CLI
---

You have access to the `job-tracker` CLI to manage job applications. Data is stored in a local SQLite database (`jobtracker.db`).

## Commands

| Command | Description |
|---------|-------------|
| `job-tracker add <company> <position>` | Add a job. Flags: `--status`, `--category`, `--salary`, `--location`, `--contact`, `--url`, `--notes`, `--date`, `--applied-date`, `--reminder` |
| `job-tracker list` | List jobs. Flags: `--status`, `--category`, `--search`, `--limit`, `--all` |
| `job-tracker get <id>` | View job details. Flag: `--history` |
| `job-tracker update <id>` | Update job. Same flags as `add` |
| `job-tracker delete <id>` | Delete a job. Flag: `--force` |
| `job-tracker stats` | Show statistics |
| `job-tracker start` | Start web UI. Flag: `--port` (default 8080) |
| `job-tracker init` | Init database. Flag: `--force` |

All commands accept `--db <path>` for a custom database and `--json` for machine-readable output.

## Database tables

- `jobs` — company, position, status, category, salary, location, contact, url, notes, dates
- `categories` — custom labels
- `history` — activity log per job
- `profile` — user name, skills, experience
- `settings` — theme, reminders, default view

## Examples

Add a job you just applied to:
`job-tracker add "Google" "Software Engineer" --status Applied --date 2026-06-20`

List active applications:
`job-tracker list --status "Not Applied" --status Applied --status Offer`

Update status when rejected:
`job-tracker update 1 --status Rejected`

Show stats:
`job-tracker stats`

Launch the dashboard:
`job-tracker start --port 8080`
