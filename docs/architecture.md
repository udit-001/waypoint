# Architecture

## Data Storage

All data lives in a SQLite database at `~/.waypoint/waypoint.db`.

| Table | Contents |
|-------|----------|
| `jobs` | Applications with company, position, status, notes |
| `categories` | Custom labels for grouping jobs |
| `artifacts` | AI-generated content with multi-variant support |
| `history` | Activity audit trail |
| `profile` | Name, skills, experience, education, brief fields |

Experience/education are TEXT columns holding a JSON array of structured
objects — `experience`: `{title, company, start, end, description}`, `education`:
`{institution, degree, start, end, description}`. Dates are partial ISO (`YYYY-MM`, or
`YYYY` when the month is unknown); an empty `end` means "present". `description`
is free-text detail (role scope, bullet points, GPA) and is optional. Legacy
flat-string arrays upgrade on read (`ParseExperienceEntries`). `DeriveSeniority`
totals date ranges (regex is a fallback for date-less entries).

Profile writes use the **document-patch pattern**: the writable surface is one
JSON document (the same shape `profile show --json` emits). `waypoint profile
schema --json` prints the empty template; `profile set --file doc.json` (CLI)
and `PATCH /api/profile` (web) patch only the keys present. Keys, validation,
and the schema template live behind the db seam (`internal/db/documents.go`),
shared by both surfaces.
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

REST API at `/api/`. All endpoints return JSON. Reads are free; the only
write route is the profile brief (`PATCH /api/profile`), which crosses the
same `db.Store` seam as the CLI — the web UI is read-only for everything else.

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
| `GET /api/brief` | Curation brief (facts/constraints/preferences + open frontier) |
| `PATCH /api/profile` | Write any profile field as a patch document (camelCase keys matching `GET /api/profile` and the CLI's `profile set --file`; `waypoint profile schema` shows the writable surface); returns the updated brief |
| `GET /api/settings` | App settings |
| `GET /api/search?q=` | Unified search across jobs and artifacts |
