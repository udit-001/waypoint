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

// LandVerified lands already-verified candidates into boards.toml — the
// auto-promote half of the promote path (WP-181). Discover's verify gate
// has already proven each board link live, so no re-probe happens here;
// the caller pays the network once. Entries are flagged Discovered and
// carry the company domain, so keep/block on the Companies page acts
// without consulting the ledger. Already-watched URLs are skipped;
// a same-name board pointing elsewhere is the same conflict Promote
// raises, and writes nothing (atomic: a conflict lands nothing). One
// entry name = one board in boards.toml, so of a candidate's verified
// boards the first lands and the rest skip — the company name links to
// the first verified board. Returns the landed companies' names, in
// candidate order, plus the number of extra verified boards skipped (one
// entry name = one board, so beyond the first they cannot land — the
// caller logs the count rather than dropping it silently).
func LandVerified(cands []Candidate, bf *config.BoardsFile) (landed []string, skipped int, err error) {
	for _, c := range cands {
		entryName := BoardEntryName(c.Name)
		wrote := false // one entry name = one board: the first verified wins
		for _, l := range c.Boards {
			if wrote {
				skipped++
				break
			}
			watched := false
			for _, e := range bf.Boards {
				if e.URL == l.URL {
					watched = true
					break
				}
			}
			if watched {
				continue
			}
			if existing := bf.Find(entryName); existing != nil && existing.URL != l.URL {
				return landed, skipped, fmt.Errorf("board %q already exists with a different URL (%s)", entryName, existing.URL)
			}
			bf.Upsert(config.BoardEntry{
				Name:       entryName,
				Company:    c.Name,
				URL:        l.URL,
				Provider:   l.Provider,
				Enabled:    true,
				Discovered: true,
				Domain:     strings.ToLower(strings.TrimSpace(c.Domain)),
				AddedAt:    time.Now().UTC().Format(time.RFC3339),
			})
			wrote = true
			landed = append(landed, c.Name)
		}
	}
	return landed, skipped, nil
}
