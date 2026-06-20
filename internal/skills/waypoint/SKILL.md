---
name: waypoint
description: Manage job applications using the waypoint CLI
---

You have access to the `waypoint` CLI to manage job applications. Data is stored in a local SQLite database (`jobtracker.db`).

## Commands

| Command | Description |
|---------|-------------|
| `waypoint add <company> <position>` | Add a job. Flags: `--status`, `--category`, `--salary`, `--location`, `--contact`, `--url`, `--notes`, `--date`, `--applied-date`, `--reminder` |
| `waypoint list` | List jobs. Flags: `--status`, `--category`, `--search`, `--limit`, `--all` |
| `waypoint get <id>` | View job details. Flag: `--history` |
| `waypoint update <id>` | Update job. Same flags as `add` |
| `waypoint delete <id>` | Delete a job. Flag: `--force` |
| `waypoint stats` | Show statistics |
| `waypoint start` | Start web UI. Flag: `--port` (default 8080) |
| `waypoint init` | Init database. Flag: `--force` |

All commands accept `--db <path>` for a custom database and `--json` for machine-readable output.

## Database tables

- `jobs` — company, position, status, category, salary, location, contact, url, notes, dates
- `categories` — custom labels
- `history` — activity log per job
- `profile` — user name, skills, experience
- `settings` — theme, reminders, default view

## Examples

Add a job you just applied to:
`waypoint add "Google" "Software Engineer" --status Applied --date 2026-06-20`

List active applications:
`waypoint list --status "Not Applied" --status Applied --status Offer`

Update status when rejected:
`waypoint update 1 --status Rejected`

Show stats:
`waypoint stats`

Launch the dashboard:
`waypoint start --port 8080`
