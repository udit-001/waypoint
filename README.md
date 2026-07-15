<p align="center">
  <img src="web/public/icons/icon-192.svg" width="64" height="64" alt="Waypoint logo">
</p>

# Waypoint — Job Tracker

A job application tracker with a Go backend and Svelte frontend.
Data mutations happen through the CLI; the web UI is a read-only dashboard.
The entire web UI is compiled into a single self-contained binary.

## Install

### Recommended: `go install` (works everywhere, no binary downloads)

```bash
go install github.com/udit-001/waypoint/cmd/waypoint@latest
```

This compiles Waypoint from source with the web UI embedded. No pre-built
binaries to download, no Windows SmartScreen warnings, no trust decisions.

### From a release (if you don't have Go installed)

```bash
curl -sfL https://raw.githubusercontent.com/udit-001/waypoint/main/install.sh | sh
```

Install a specific version:

```bash
curl -sfL https://raw.githubusercontent.com/udit-001/waypoint/main/install.sh | sh -s -- v0.4.0
```

### From source

```bash
git clone https://github.com/udit-001/waypoint.git
cd waypoint
make build     # builds frontend + Go binary
```

Or step by step:

```bash
cd web && pnpm install && pnpm build && cd ..
go build -o waypoint ./cmd/waypoint
```

### How it works

The Svelte frontend is pre-built and committed to `web/dist/`, which is embedded
into the Go binary at compile time via `//go:embed`. This means:
- `go install` downloads source + pre-built frontend from git → fully functional binary
- No `node_modules` or build tools needed to install
- Frontend rebuild is only needed when modifying UI code

### Upgrade

```bash
waypoint upgrade
```

This runs `go install github.com/udit-001/waypoint/cmd/waypoint@latest` internally,
stopping and restarting the server if it's running.

## Quick Start

```bash
waypoint init
waypoint start
```

Opens at `http://localhost:8080`. Use `--port` to change the port.

## Usage

Waypoint is CLI-first. Add, update, and delete jobs from the terminal.
The web UI is a read-only dashboard for what you've tracked.

```bash
waypoint jobs add "Google" "Senior SWE" --status Applied
waypoint jobs add "Meta" "Staff Engineer" --status "Not Applied"
waypoint jobs stats
waypoint jobs list
```

Full CLI reference at [docs/cli.md](docs/cli.md).

### Key commands at a glance

- **`jobs add/list/get/update/delete/stats`** — Track applications
- **`profile show/set`** — Personal info for AI content generation
- **`artifacts add/list/get/delete/archive`** — Save generated content
- **`categories list/add/rename/delete`** — Organize jobs into groups
- **`init/start/stop`** — System management

All commands accept `--db <path>` and `--json`.

## AI Integration

Waypoint ships a skill file that teaches AI coding assistants
how to use the CLI and generate job-search content.

```bash
waypoint skills install --agent pi.dev
```

Supported agents: `pi.dev`, `claude-code`, `codex`, `opencode`.

### Built-in generation skills

| Skill | What it produces |
|-------|-----------------|
| Email Generator | Application, follow-up, thank-you, networking (4 tones) |
| Cover Letter Generator | Formal, casual, creative, executive styles |
| Resume Keyword Optimizer | Match score, gap analysis, verb suggestions |
| Interview Prep Assistant | Role-specific Q&A, research checklist |
| Career Summary Generator | 5 resume summary styles |
| Statement of Purpose Generator | SOP for grad school, fellowships, research |

When an AI agent generates content, it saves it as an artifact.
Artifacts store every variant (tones, lengths, styles) and link to the job.

```bash
waypoint artifacts add --skill cover-letter --title "Cover" -f /tmp/cover.txt --job 3
waypoint artifacts list --job 3
waypoint artifacts get 12
```

## Web UI

Read-only dashboard with 9 views. Manage data via CLI.

- **Dashboard** — Stats cards + charts (status, category, monthly trend)
- **Kanban** — Columnar board grouped by status
- **Table** — Sortable table with category filter
- **Search** — Full-text search across jobs and artifacts
- **Categories** — Manage categories
- **Profile** — View personal info, skills, education
- **AI Integration** — Browse skills, install command
- **Artifacts** — Browse generated content with variant tabs
- **Settings** — Typography, CLI reference, app settings

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `/` | Focus search |

## Data

SQLite at `~/.waypoint/waypoint.db`. 7 tables: jobs, categories, artifacts,
history, profile, settings, FTS5 indices.

See [docs/architecture.md](docs/architecture.md) for the full schema,
tech stack, API endpoints, and project layout.

## License

MIT
