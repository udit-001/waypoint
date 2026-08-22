package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
)

// runDiscovery is the seam between the command and the discovery
// module: a variable so CLI tests stub the network-touching pipeline.
var runDiscovery = discovery.Discover

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover candidate companies and their ATS boards",
	Long: `Map the company universe onto watchable ATS boards.

Discover probes each candidate company's careers pages, extracts ATS
board links (Greenhouse, Lever, Ashby, Workday, Eightfold), verifies the
boards are live through the same detection used by 'boards add', and
persists survivors as candidates for review.

Already-watched boards are filtered out, so a company you already track
never surfaces again. Review vocabulary: suggested (awaiting a decision),
added (promoted to boards.toml), dismissed (permanently skipped).`,
}

// strconvFormatID renders a candidate id for the table.
func strconvFormatID(n int64) string { return strconv.FormatInt(n, 10) }

// boardShort renders one board compactly for table output. The token
// that matters differs per provider: tenant for Eightfold, site slug
// for Workday, board token/org for the rest.
func boardShort(l discovery.BoardLink) string {
	part := ""
	if u, err := url.Parse(l.URL); err == nil {
		switch l.Provider {
		case "eightfold":
			part = strings.TrimSuffix(u.Hostname(), ".eightfold.ai")
		default:
			segs := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(segs) > 0 && segs[0] != "" {
				part = segs[0]
			}
		}
	}
	return fmt.Sprintf("%s/%s", l.Provider, part)
}

var discoverRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Probe the facet list and persist discovered companies",
	Long: `Probe each facet company's careers pages and persist the ones with
working ATS boards as candidates (status "suggested") for review.

Already-watched board URLs are filtered out, and companies with a
review decision are never re-probed — re-runs surface only genuinely
new suggestions, without duplicating rows or resetting decisions.

Each candidate reports an id plus its verified board URL(s); those ids
are what 'discover add' and 'discover dismiss' consume. Run is done
when every suggestion has been reviewed — nothing remains suggested.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, _, err := loadBoardsStore()
		if err != nil {
			return err
		}
		watched := make(map[string]bool, len(bf.Boards))
		for _, b := range bf.Boards {
			watched[b.URL] = true // discovery normalizes case/slashes itself
		}

		// Companies with a review decision (added/dismissed) are not
		// re-probed; only still-suggested rows earn fresh fetches.
		persisted, err := store.Candidates("")
		if err != nil {
			return formatError("list candidates", err)
		}
		decided := make(map[string]bool, len(persisted))
		statusByName := make(map[string]string, len(persisted))
		for _, c := range persisted {
			statusByName[c.Name] = c.Status
			if c.Status != db.StatusCandidateSuggested {
				decided[c.Name] = true
			}
		}

		facets := discovery.HardcodedFacets()
		if len(decided) > 0 {
			filtered := make([]discovery.Facet, 0, len(facets))
			for _, f := range facets {
				kept := f.Companies[:0]
				for _, c := range f.Companies {
					if !decided[c.Name] {
						kept = append(kept, c)
					}
				}
				f.Companies = kept
				filtered = append(filtered, f)
			}
			facets = filtered
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		found, err := runDiscovery(ctx, facets, watched, discovery.Options{})
		if err != nil {
			return formatError("discover", err)
		}

		toSave := make([]db.CompanyCandidate, 0, len(found))
		for _, c := range found {
			bs := make([]db.CandidateBoard, 0, len(c.Boards))
			for _, l := range c.Boards {
				bs = append(bs, db.CandidateBoard{Provider: l.Provider, URL: l.URL})
			}
			toSave = append(toSave, db.CompanyCandidate{
				Name: c.Name, Domain: c.Domain, Facet: c.Facet,
				Boards: bs, Status: db.StatusCandidateSuggested,
			})
		}
		if err := store.SaveCandidates(toSave); err != nil {
			return formatError("save candidates", err)
		}

		// Re-read after save so rows show true post-save boards/status.
		persistedAfter, err := store.Candidates("")
		if err != nil {
			return formatError("list candidates", err)
		}
		for _, c := range persistedAfter {
			statusByName[c.Name] = c.Status
		}

		type row struct {
			ID     int64               `json:"id"`
			Name   string              `json:"name"`
			Domain string              `json:"domain"`
			Facet  string              `json:"facet"`
			Boards []db.CandidateBoard `json:"boards"`
			Status string              `json:"status"`
		}
		out := make([]row, 0, len(found))
		for _, c := range found {
			r := row{Name: c.Name, Domain: c.Domain, Facet: c.Facet, Status: statusByName[c.Name]}
			if pc, ok := findCandidate(persistedAfter, c.Name); ok {
				r.ID = pc.ID
				r.Boards = pc.Boards
			}
			out = append(out, r)
		}

		if jsonOut {
			printJSON(map[string]any{
				"meta":       map[string]any{"facets": len(facets), "probed": len(found), "found": len(out)},
				"candidates": out,
			})
			return nil
		}

		fmt.Println()
		if len(out) == 0 {
			fmt.Println("  No new companies found. Everything mapped is either live on your boards or previously dismissed.")
			fmt.Println()
			return nil
		}
		rows := make([][]string, 0, len(out))
		for _, r := range out {
			shorts := make([]string, 0, len(r.Boards))
			for _, b := range r.Boards {
				shorts = append(shorts, boardShort(discovery.BoardLink{Provider: b.Provider, URL: b.URL}))
			}
			rows = append(rows, []string{strconvFormatID(r.ID), r.Name, r.Facet, strings.Join(shorts, ", "), r.Status})
		}
		fmt.Println(formatTable([]string{"ID", "Company", "Facet", "Boards", "Status"}, rows))
		fmt.Printf("\n  %d company(ies) discovered and saved to the discovery ledger.\n\n", len(out))
		return nil
	},
}

// findCandidate locates one persisted candidate by name.
func findCandidate(cands []db.CompanyCandidate, name string) (db.CompanyCandidate, bool) {
	for _, c := range cands {
		if c.Name == name {
			return c, true
		}
	}
	return db.CompanyCandidate{}, false
}

func init() {
	discoverCmd.AddCommand(discoverRunCmd)
	rootCmd.AddCommand(discoverCmd)
}
