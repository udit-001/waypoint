<p align="center">
  <img src="web/icons/icon-192.svg" width="64" height="64" alt="Waypoint logo">
</p>

# Waypoint — Job Tracker

A job application tracker with a Go backend (SQLite + REST API) and a pure vanilla JS frontend.  
Data mutations happen through the CLI; the web UI is a read-only dashboard.

## Install

### From a release (curl pipe)

```bash
curl -sfL https://raw.githubusercontent.com/SwatiBio/waypoint/main/install.sh | sh
```

To install a specific version:

```bash
curl -sfL https://raw.githubusercontent.com/SwatiBio/waypoint/main/install.sh | sh -s -- v0.2.1
```

### With Go (recommended)

```bash
go install github.com/SwatiBio/waypoint/cmd/waypoint@latest
```

### From source

```bash
git clone https://github.com/SwatiBio/waypoint.git
cd waypoint
go build -o waypoint ./cmd/waypoint
```

## Quick Start

```bash
# Initialize the database
waypoint init

# Start the web UI (opens on http://localhost:8080)
waypoint start
```

## CLI Commands

### Jobs

| Command | Description |
|---------|-------------|
| `waypoint jobs add <company> <position>` | Add a job. Flags: `--status`, `--category`, `--salary`, `--location`, `--contact`, `--url`, `--notes`, `--date`, `--applied-date`, `--reminder` |
| `waypoint jobs list` | List jobs. Flags: `--status`, `--category`, `--search`, `--limit`, `--all` |
| `waypoint jobs get <id>` | Show job details. Flag: `--history` |
| `waypoint jobs update <id>` | Update job fields. Same flags as `add` |
| `waypoint jobs delete <id>` | Delete a job. Flag: `--force` |
| `waypoint jobs stats` | Show aggregate statistics |

### Categories

| Command | Description |
|---------|-------------|
| `waypoint categories list` | List all categories with job counts |
| `waypoint categories add <name>` | Add a new category |
| `waypoint categories rename <id> <name>` | Rename a category by ID |
| `waypoint categories delete <id>` | Delete a category by ID (jobs move to General) |

Alias: `waypoint cat`

### Profile

| Command | Description |
|---------|-------------|
| `waypoint profile show` | Display your profile (`--json` for machine output) |
| `waypoint profile set` | Update profile fields. Flags: `--name`, `--email`, `--phone`, `--title`, `--skills` (JSON array), `--experience`, `--education`, `--industry`, `--greeting-style`, `--sign-off` |

### Artifacts

| Command | Description |
|---------|-------------|
| `waypoint artifacts list` | List generated content. Flags: `--skill`, `--job`, `--all` |
| `waypoint artifacts get <id>` | Show artifact with all variants |
| `waypoint artifacts delete <id>` | Delete an artifact. Flag: `--force` |
| `waypoint artifacts archive <id>` | Soft-delete (hide from default list) |

Alias: `waypoint artifact`

### Other

| Command | Description |
|---------|-------------|
| `waypoint init` | Initialize a new SQLite database. Flag: `--force` |
| `waypoint start` | Launch the web UI server. Flag: `--port` (default 8080) |
| `waypoint stop` | Stop the background web UI server |
| `waypoint skills install` | Install agent skill for AI coding assistants. Flag: `--agent` |
| `waypoint upgrade` | Self-update to the latest release |

All commands support `--db <path>` and `--json`.

## Web UI

Nine views, all read-only (mutations via CLI):

- **Dashboard** — Stats cards + charts (status doughnut, category bar, monthly trend)
- **Kanban** — Columnar board grouped by status
- **Table** — Sortable table with category filter (pills or dropdown)
- **Timeline** — Chronological activity history
- **Categories** — Manage categories table with CLI actions
- **Profile** — View personal info, skills, education, experience
- **AI Integration** — Browse 6 built-in skills + install command for AI agents
- **Artifacts** — Browse AI-generated content with variant tabs and job linking
- **Settings** — Typography, CLI reference, app settings

Breadcrumb navigation in the top bar for detail pages (job → artifacts).

## AI Skills

Waypoint ships a skill file that teaches AI coding assistants how to use the CLI and generate job-search content. Install it with:

```bash
waypoint skills install --agent pi.dev
```

Supported agents: `pi.dev`, `claude-code`, `codex`, `opencode`

### Built-in generation skills

| Skill | Generates |
|-------|-----------|
| Email Generator | Application, follow-up, thank-you, networking emails (4 tones) |
| Cover Letter Generator | Cover letters in formal, casual, creative, executive styles |
| Resume Keyword Optimizer | Match score, gap analysis, action-verb suggestions |
| Interview Prep Assistant | Role-specific questions, sample answers, research checklist |
| Career Summary Generator | Resume summaries in 5 styles (standard, impact, technical, executive, entry-level) |
| Statement of Purpose Generator | SOPs for grad school, fellowships, research programs (4 tones) |

## Tech Stack

- **Backend:** Go 1.25 — standard library `net/http`, REST API, embedded static files
- **CLI:** Cobra CLI framework
- **Database:** SQLite (via `modernc.org/sqlite` — pure Go, no CGo)
- **Frontend:** Vanilla HTML/CSS/JS (ES6+), no frameworks
- **Charts:** Chart.js 4.4.1
- **Markdown:** marked 11.1.1
- **Typography:** Inter & PT Serif
- **PWA:** Service worker for offline caching

## Data Storage

All data lives in a SQLite database (`~/.waypoint/waypoint.db`). Tables:

- `jobs` — Applications with company, position, status, category (FK), notes, etc.
- `categories` — Custom category labels (FK on jobs)
- `artifacts` — AI-generated content with multi-variant support, linked to jobs
- `history` — Activity log (action audit trail)
- `profile` — User profile (name, skills, experience, etc.)
- `settings` — App preferences (theme, reminders, default view)

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+N` | New job (opens CLI hint) |
| `Ctrl+F` | Focus search |
| `Ctrl+S` | Export data |

## License

MIT
