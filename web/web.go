// Package web embeds the pre-built Svelte frontend into the Go binary.
//
// The frontend dist/ directory is tracked in git so that go install works
// without requiring Node.js at install time. To rebuild the frontend:
//
//	cd web && pnpm install && pnpm build
//
// Or from the project root:
//
//	go generate ./web/...
package web

import "embed"

//go:generate sh -c "cd web && pnpm install --frozen-lockfile && pnpm build"

//go:embed dist
var Files embed.FS
