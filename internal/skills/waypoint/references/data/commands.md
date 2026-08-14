# Command Reference

All commands: `--json`.

## Jobs

| Cmd | Flags |
|-----|-------|
| `jobs add <co> <pos>` | `--status` `--category` `--salary` `--location` `--contact` `--url` `--notes` `--date` `--applied-date` `--reminder` |
| `jobs list` | `--status` `--category` `--search` `--limit` `--all` |
| `jobs get <id>` | `--history` |
| `jobs update <id>` | same flags as `add` |
| `jobs delete <id>` | `--force` |
| `jobs stats` | |

## Artifacts

| Cmd | Flags |
|-----|-------|
| `artifacts add` | `--skill` `--title` `--title-file` `-f` `--variant-content` `--variant-file` `--variant-label` `--variants` `--variants-file` `--options` `--options-file` `--job` |
| `artifacts list` | `--skill` `--job` `--all` |
| `artifacts get <id>` | |
| `artifacts delete <id>` | `--force` |
| `artifacts archive <id>` | |

## Other

| Cmd | Flags |
|-----|-------|
| `categories list\|add\|rename\|delete` | |
| `profile show\|set` | `--name` `--email` `--phone` `--title` `--skills` `--experience` `--education` `--industry` `--experience-file` `--education-file` `--current-location` `--seniority` `--visa-sponsorship` `--salary-floor` `--remote` `--location-preference` `--companies` `--avoid-companies` `--keywords` `--dealbreakers` |

Shell-safety rule: JSON object arrays (experience/education) are shell-unsafe
in bash, PowerShell, and cmd — use `--experience-file`/`--education-file`
(write the file with your file tool, never a shell heredoc), not the inline
flags. List fields (`--skills`, `--companies`, `--keywords`, …) take a
shell-safe comma-separated list; the JSON-array form is for programmatic use.
| `start` | `--port` (8080) |
| `init` | `--force` |
