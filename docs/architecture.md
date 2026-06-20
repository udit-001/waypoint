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
- **Database:** SQLite (pure Go via `modernc.org/sqlite`, no CGO)
- **Frontend:** Svelte 5, Vite 8, Tailwind CSS 4, Chart.js 4.5
- **Frontend embed:** Embedded into Go binary via `//go:embed` — `web/dist/` is tracked in git
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
├── web/                       # Svelte frontend
│   ├── src/                   # Svelte components & app code
│   ├── dist/                  # Pre-built output (tracked in git)
│   ├── public/                # Static assets (icons, sw.js)
│   ├── web.go                 # //go:embed dist entry point
│   ├── vite.config.js
│   └── package.json
├── Makefile                   # Build automation
└── docs/                      # Documentation
```

## Frontend Embedding

The Svelte frontend is pre-built into `web/dist/` and checked into git.
At compile time, `web/web.go` uses `//go:embed dist` to include all
assets in the binary. This makes `go install` produce a fully functional
binary without requiring Node.js at install time.

To rebuild the frontend during development:

```bash
cd web && pnpm install && pnpm build
# or
make frontend
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
