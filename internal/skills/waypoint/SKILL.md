---
name: waypoint
description: Manage job applications using the waypoint CLI
---

Manage job applications with the `waypoint` CLI. Data in local SQLite (`jobtracker.db`).

## Commands

| Cmd | Does | Flags |
|-----|------|-------|
| `jobs add <company> <position>` | add job | `--status` `--category` `--salary` `--location` `--contact` `--url` `--notes` `--date` `--applied-date` `--reminder` |
| `jobs list` | list jobs | `--status` `--category` `--search` `--limit` `--all` |
| `jobs get <id>` | job details | `--history` |
| `jobs update <id>` | update | same as `add` |
| `jobs delete <id>` | delete | `--force` |
| `jobs stats` | stats | |
| `categories` | manage categories | subcommands: `list` `add <name>` `rename <id> <name>` `delete <id>` |
| `profile` | manage profile | subcommands: `show` `set` (name, email, title, skills, etc.) |
| `start` | web UI | `--port` (8080) |
| `init` | init db | `--force` |

All cmds: `--db <path>`, `--json`.

## Tables
`jobs` · `categories` · `history` · `profile` (name, skills, exp) · `settings`

## Generation references

Job-search content generation. Load on demand — each pulls job + profile via CLI, outputs drafted content.

| Ref | Use for |
|-----|---------|
| [email-generator](references/email-generator.md) | application / follow-up / thank-you / networking emails |
| [cover-letter](references/cover-letter.md) | cover letters (formal, casual, creative, exec) |
| [resume-optimizer](references/resume-optimizer.md) | keyword match score + gap analysis vs a posting |
| [interview-prep](references/interview-prep.md) | interview questions, answers, research checklist |
| [career-summary](references/career-summary.md) | resume summary / professional bio |

When asked for that content, `read` the matching reference, then `waypoint jobs get <id>` for fresh data.

## Examples
Add applied job → `waypoint jobs add "Google" "SWE" --status Applied --date 2026-06-20`
Active apps → `waypoint jobs list --status Applied --status Offer`
Mark rejected → `waypoint jobs update 1 --status Rejected`
Stats → `waypoint jobs stats`
Dashboard → `waypoint start --port 8080`
