# Job Application Tracker

A Notion-like job application tracker with a Go backend (SQLite + REST API) and a pure vanilla JS frontend.  
Data mutations happen through the CLI; the web UI is a read-only dashboard.

## Install

### From a release (curl pipe)

```bash
curl -sfL https://raw.githubusercontent.com/SwatiBio/Job-tracker/main/install.sh | sh
```

To install a specific version:

```bash
curl -sfL https://raw.githubusercontent.com/SwatiBio/Job-tracker/main/install.sh | sh -s -- v0.1.0
```

### From source

```bash
git clone https://github.com/SwatiBio/Job-tracker.git
cd Job-tracker
go build -o job-tracker ./cmd/job-tracker
```

## Quick Start

```bash
# Initialize the database
./job-tracker init

# Start the web UI (opens on http://localhost:8080)
./job-tracker start
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize a new SQLite database |
| `add <company> <position>` | Add a job application |
| `list` | List jobs (flags: `--status`, `--category`, `--search`, `--limit`, `--all`) |
| `get <id>` | Show job details (`--history` for activity log) |
| `update <id>` | Update job fields (`--status`, `--company`, `--position`, etc.) |
| `delete <id>` | Delete a job (`--force` to skip confirmation) |
| `stats` | Show aggregate statistics |
| `start` | Launch the web UI server |

All commands support `--db` (database path) and `--json` (JSON output).

## Web UI

Seven read-only views:

- **Dashboard** — Stats cards + charts (status doughnut, category bar, monthly trend)
- **Kanban** — Columnar board grouped by status
- **Table** — Sortable table with column filters
- **Timeline** — Chronological activity history
- **Skills** — 5 built-in generators (email, cover letter, resume optimizer, interview prep, career summary)
- **Generated Content** — Browse saved outputs
- **Settings** — View profile, settings, and AI integration config

The web UI fetches data from the Go REST API (`GET /api/*`). All mutations (add/update/delete) require the CLI.

## AI Integration (Optional)

1. Get a free API key from [Google AI Studio](https://aistudio.google.com/apikey)
2. Set it via CLI: `job-tracker update settings --gemini-key <key>`
3. Generators will use Gemini AI with automatic fallback to built-in templates

## Tech Stack

- **Backend:** Go 1.25 — standard library `net/http`, REST API, embedded static files
- **CLI:** Cobra CLI framework
- **Database:** SQLite (via `modernc.org/sqlite` — pure Go, no CGo)
- **Frontend:** Vanilla HTML/CSS/JS (ES6+), no frameworks
- **Charts:** Chart.js 4.4.1 (`web/vendor/`)
- **Markdown:** marked 11.1.1 (`web/vendor/`)
- **Typography:** PT Serif (`web/fonts/`)
- **PWA:** Service worker for offline caching

## Data Storage

All data lives in a SQLite database (`jobtracker.db`). Tables:

- `jobs` — Applications with company, position, status, notes, etc.
- `categories` — Custom category labels
- `history` — Activity log (action audit trail)
- `profile` — User profile (name, skills, experience, etc.)
- `settings` — App preferences (theme, reminders, default view)

Export from Settings → Export/Import (JSON download).

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+N` | New job (opens CLI hint) |
| `Ctrl+F` | Focus search |
| `Ctrl+S` | Export data |

## License

MIT
