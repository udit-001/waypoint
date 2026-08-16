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
│   ├── mcp/                   # MCP Streamable HTTP client (JSON-RPC 2.0 + SSE) — same pattern as income-tracker
│   ├── linkedin/              # LinkedIn profile fetch via Exa MCP + markdown parser (powers /api/profile/import-linkedin)
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
assets in the binary. Consequences:
- `go install` downloads source + pre-built frontend from git → fully
  functional binary, no Node.js at install time
- No pre-built binaries to trust, no platform-specific download matrices
- Frontend rebuild is only needed when UI code changes

To rebuild the frontend during development:

```bash
cd web && pnpm install && pnpm build
# or
make frontend
```

## Building from Source

```bash
git clone https://github.com/udit-001/waypoint.git
cd waypoint
make build     # frontend + Go binary → bin/waypoint
make check     # pre-commit gate: gofmt, go vet, go + frontend tests
make dev       # backend + Vite dev server with live proxy
```

`make start` / `make stop` run the web UI as a background daemon during
development. `make test`, `make fmt`, `make tidy`, `make clean` also exist —
see the Makefile. CGO is always disabled (`CGO_ENABLED=0`); the SQLite
driver is pure Go and no CGO dependency may be introduced.

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
| `POST /api/profile/import-linkedin` | Fetch a public LinkedIn profile via Exa's hosted MCP server (`web_fetch_exa`) and **merge** it into the stored profile — **never writes**. Returns `{doc, summary}`: `doc` is the merged profile document (the same camelCase keys `PATCH /api/profile` accepts), `summary` is an Added/Updated/Kept diff (`experienceAdded/Updated`, `educationAdded/Updated`, `skillsAdded`, `*Kept` counts). The merge never deletes: entries match by (title, company) for experience and by institution for education; matched entries get dates/description updated from LinkedIn, unmatched fetched entries are appended, existing entries with no match are kept. An empty stored profile merges to the fetched profile (everything "added"), so seed and update share one code path. The web UI previews the diff and PATCHes `doc` on Apply. `{url}` in the body; 400 on a non-LinkedIn/`/in/` URL, 502 on fetch failure, 422 when the page yielded nothing parseable (login wall / private profile) |
| `GET /api/settings` | App settings |
| `GET /api/search?q=` | Unified search across jobs and artifacts |
