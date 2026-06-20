# Architecture

## Data Storage

All data lives in a SQLite database at `~/.waypoint/waypoint.db`.

| Table | Contents |
|-------|----------|
| `jobs` | Applications with company, position, status, notes |
| `categories` | Custom labels for grouping jobs |
| `artifacts` | AI-generated content with multi-variant support |
| `history` | Activity audit trail |
| `profile` | Name, skills, experience, education |
| `settings` | Theme, default view, reminders |
| `jobs_fts` / `artifacts_fts` | FTS5 full-text search indices |

## Tech Stack

- **Backend:** Go 1.25 — standard library `net/http`, REST API
- **CLI:** Cobra CLI framework
- **Database:** SQLite (pure Go via `modernc.org/sqlite`)
- **Frontend:** Vanilla HTML/CSS/JS (ES6+), no frameworks
- **Charts:** Chart.js 4.4.1
- **Markdown:** marked 11.1.1
- **Typography:** Inter & PT Serif
- **PWA:** Service worker for offline caching

## Project Layout

```
├── cmd/waypoint/main.go       # Entry point
├── internal/
│   ├── cli/                   # Cobra commands (jobs, artifacts, etc.)
│   ├── db/                    # SQLite models, queries, FTS5
│   ├── server/                # HTTP server, API handlers
│   ├── skills/                # AI skill definitions
│   └── version/               # Build version
├── web/                       # Static frontend (HTML, JS, CSS)
│   ├── css/
│   ├── js/
│   ├── fonts/
│   └── icons/
└── docs/                      # Documentation
```

## API

Read-only REST API at `/api/`. All endpoints return JSON.

| Endpoint | Returns |
|----------|---------|
| `GET /api/jobs` | All jobs (filterable: `?search=`, `?status=`, `?category=`) |
| `GET /api/jobs/{id}` | Single job |
| `GET /api/jobs/{id}/history` | Activity log for a job |
| `GET /api/stats` | Aggregate statistics |
| `GET /api/history` | All activity |
| `GET /api/categories` | All categories |
| `GET /api/artifacts` | All artifacts (filterable: `?skill=`, `?job=`, `?search=`) |
| `GET /api/artifacts/{id}` | Single artifact with all variants |
| `GET /api/profile` | User profile |
| `GET /api/settings` | App settings |
| `GET /api/search?q=` | Unified search across jobs and artifacts |
