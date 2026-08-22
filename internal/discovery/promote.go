package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/boards"
	"github.com/udit-001/waypoint/internal/config"
)

// ProbeBoard is the network seam behind Promote's verify gate: fetch
// page 1 and report how many postings answered. Swapped in tests —
// production delegates to boards.Probe.
var ProbeBoard = func(ctx context.Context, l BoardLink) (int, error) {
	b := boards.Board{URL: l.URL, Enabled: true}
	p, hit, err := boards.DetectProvider(b)
	if err != nil {
		return 0, fmt.Errorf("no provider matched %s", l.URL)
	}
	return boards.Probe(ctx, p, b, hit)
}

// PromoteOutcome reports what one promotion changed.
type PromoteOutcome struct {
	Added          int
	AlreadyWatched int
	Fetched        int    // first-page jobs across added boards
	Provider       string // provider of the last board added
}

// Promote is the single source of the candidate-promotion path, shared
// by 'discover add' (CLI) and POST /api/candidates/{id}/add (web): run
// each of the candidate's boards through the verify gate — page 1 must
// answer before anything is written — then land survivors as enabled
// entries in bf. A URL already watched counts as AlreadyWatched; a
// same-name board pointing elsewhere is a conflict error that writes
// nothing. The caller saves bf and flips the candidate's status.
func Promote(ctx context.Context, companyName string, links []BoardLink, bf *config.BoardsFile) (PromoteOutcome, error) {
	if len(links) == 0 {
		return PromoteOutcome{}, fmt.Errorf("candidate has no boards to promote")
	}

	entryName := BoardEntryName(companyName)
	var out PromoteOutcome
	for _, l := range links {
		watched := false
		for _, e := range bf.Boards {
			if e.URL == l.URL {
				watched = true
				break
			}
		}
		if watched {
			out.AlreadyWatched++
			continue
		}

		entry := config.BoardEntry{
			Name:    entryName,
			Company: companyName,
			URL:     l.URL,
			Enabled: true,
			AddedAt: time.Now().UTC().Format(time.RFC3339),
		}
		// A same-name board pointing elsewhere must never be silently
		// replaced — surface the conflict instead.
		if existing := bf.Find(entry.Name); existing != nil {
			return PromoteOutcome{}, fmt.Errorf("board %q already exists with a different URL (%s)", entry.Name, existing.URL)
		}

		n, err := ProbeBoard(ctx, l)
		if err != nil {
			return PromoteOutcome{}, fmt.Errorf("verification failed for %s: %w", l.URL, err)
		}

		entry.Provider = l.Provider
		bf.Upsert(entry)
		out.Added++
		out.Fetched += n
		out.Provider = l.Provider
	}
	return out, nil
}

// BoardEntryName derives the boards.toml entry name from a company's
// display name: lowercase, spaces to dashes — deterministic, so
// re-reviews of one company always map to one entry name.
func BoardEntryName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
}
