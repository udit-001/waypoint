# AGENTS.md

## Orientation

Waypoint is a job-application tracker: Go backend (cobra CLI + REST server,
pure-Go SQLite via `modernc.org/sqlite`, **no CGO**) with a Svelte 5/Vite 8
frontend **embedded** via `//go:embed web/dist`. `web/dist/` is committed, so
`go build` alone produces a working binary — Node/pnpm only matter when
frontend source changes.

The load-bearing **seam** is `db.Store` (`internal/db/store.go`): the CLI, the
HTTP server, and the tests all cross it. `PATCH /api/profile` — the web's only
write route — goes through the same `db.Store` methods as the CLI (see
`internal/server/profile.go`). Shared profile logic (normalization, derived
seniority, structured experience/education parsing, date math) lives **behind
the seam** in `internal/db/`, never in the adapters. The profile format lives
in `docs/architecture.md`; don't restate it here.

This file is for **contributors**. The end-user skill at
`internal/skills/waypoint/SKILL.md` drives the job-search pipeline — different
audience, don't conflate.

## Essential commands

| Task | Command |
|---|---|
| Build everything (frontend + Go) | `make build` |
| Build frontend only | `make frontend` |
| Run Go tests | `make test` |
| Run frontend tests | `make test-frontend` |
| Pre-commit gate (fmt check + vet + all tests) | `make check` |
| Format Go code | `make fmt` |
| Dev server (backend + Vite proxy) | `make dev` (start Vite separately: `cd web && pnpm dev`) |
| Tidy modules | `make tidy` |
| Clean | `make clean` |

## Conventions

- **Commits:** conventional commits **with scope** — `feat(cli): ...`, `fix(scraper): ...`, `refactor(pwa): ...`, `chore:`, `docs:`, `test(db):`, `ci:`. See `git log` for the established scopes (`cli`, `scraper`, `db`, `pwa`, `dates`, `waypoint`).
- **Branches:** `feat/<topic>` or `feat/wp-<n>-<topic>` (e.g. `feat/wp-67-ipu-filtering`). Default branch is `main`.
- **CGO:** always `CGO_ENABLED=0` — the SQLite driver is pure Go. Never introduce a CGO dependency.
- **`web/dist/` is generated** — edit `web/src/`, rebuild with `make frontend`, commit the result. Never hand-edit `web/dist/`.
- **All commands accept `--json`** — match this on new commands.

## Adding a scraper

The scraper registry is compile-time: each package self-registers in `init()`. Pattern (see `internal/scraper/iisc/iisc.go` for a canonical listing example, `internal/scraper/google/google.go` for an API example):

1. New package `internal/scraper/<name>/<name>.go` + `<name>_test.go`.
2. Struct with a `Fetcher scraper.Fetcher` field (the seam that makes it testable).
3. `init()` calls `scraper.Register(<Name>{Fetcher: &scraper.HTTPFetcher{}})`.
4. Implement the `Scraper` interface: `Name() string` (lowercase id), `Source() string` (display name), `Categories() []string`, `Search(ctx, opts) ([]Result, error)`.
5. **Listing scraper** (HTML page): fetch, parse, then call `scraper.FilterByQuery(results, opts.Query)` client-side. Optionally `scraper.FilterByRecency` if the page has no date filter. Use `scraper.CleanHTML` to strip tags.
6. **API scraper** (LinkedIn/Indeed/Google Jobs): filter server-side via query params — do **not** call `FilterByQuery`/`FilterByRecency` (double-filtering bug). See `internal/scraper/google/google.go`.
7. Optional `Detailer` interface (`Detail(ctx, id) (*Result, error)`) if the portal has a detail endpoint — the CLI type-asserts it.
8. Test: a `mockFetcher` returning a `testHTML` const fixture, assert parsing. See `iisc_test.go:10-32`.

**Done when:** `go test ./internal/scraper/<name>/` passes, `scraper.Get("<name>")` returns the scraper, and `waypoint scrape <name>` returns results against the live portal.

## Adding a CLI command

Pattern (see `internal/cli/stats.go` for a canonical read command, `internal/cli/add.go` for a write command):

1. New file `internal/cli/<name>.go`.
2. `var <name>Cmd = &cobra.Command{Use, Short, Long, Args, RunE}`.
3. `init()` calls `rootCmd.AddCommand(<name>Cmd)` — or the relevant parent (e.g. `jobsCmd` for `waypoint jobs <sub>`).
4. Flags via `cmd.Flags()...`; honor the universal `--json` (the `jsonOut` var) convention.
5. Persistence: the root `PersistentPreRunE` opens `store` (a `db.Store`) for every command except `init`/`help`/`completion`/`version`. Use `store` directly; it closes in `PersistentPostRunE`.
6. Errors via `formatError("failed to ...", err)`; JSON output via `printJSON(x)` when `jsonOut`.
7. Test in `<name>_test.go` using a `FakeStore` (see `internal/db/` for the `Store` interface + fake).

**Done when:** `go test ./internal/cli/` passes, the command appears in `waypoint --help`, and `--json` output is valid JSON.

## Where to find the rest

- `docs/architecture.md` — DB schema, tech stack, project layout, REST API, and the canonical profile/experience/education format.
- `docs/cli.md` — complete CLI command reference.
- `workflows/release.md` — the release ritual (migrate to goreleaser + cosign + nfpm, mirroring `../learn-tool`).
- `internal/scraper/scraper.go` — the `Scraper`/`Detailer` interfaces, `Register`, and filter helpers.
- `internal/version/version.go` — version var (bumped in sync on release; overridden by ldflags at build time).
- `internal/skills/waypoint/SKILL.md` — product-usage skill for end-users (the `enroll → enrich → generate → save` pipeline), embedded into the binary via `internal/skills/embed.go`. Read this only if working on the job-search flow, not on the codebase.
- `internal/resume/` — deep module for `waypoint resume extract|doctor`: pdf_oxide lib download (`lib.go`), backend selection + poppler fallback (`extract.go`), deterministic email/phone redaction (`redact.go`). Interfaces and report types are the seam; internal function-variable seams serve package tests. `resume` commands output **always-JSON** (deliberate divergence from the `--json` convention: the flag is accepted as a no-op) since agents parse them directly.

## Before you commit

- `make check` green.
