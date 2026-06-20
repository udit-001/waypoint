---
name: waypoint
description: Manage job applications using the waypoint CLI
---

Manage job applications with the `waypoint` CLI. Data in local SQLite (`jobtracker.db`).

## Commands

| Cmd | Does | Flags |
|-----|------|-------|
| `add <company> <position>` | add job | `--status` `--category` `--salary` `--location` `--contact` `--url` `--notes` `--date` `--applied-date` `--reminder` |
| `list` | list jobs | `--status` `--category` `--search` `--limit` `--all` |
| `get <id>` | job details | `--history` |
| `update <id>` | update | same as `add` |
| `delete <id>` | delete | `--force` |
| `stats` | stats | |
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

When asked for that content, `read` the matching reference, then `waypoint get <id>` for fresh data.

## Examples
Add applied job → `waypoint add "Google" "SWE" --status Applied --date 2026-06-20`
Active apps → `waypoint list --status Applied --status Offer`
Mark rejected → `waypoint update 1 --status Rejected`
Stats → `waypoint stats`
Dashboard → `waypoint start --port 8080`
