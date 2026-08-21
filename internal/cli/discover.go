package cli

import (
	"context"
	"fmt"
	"net/url"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, _, err := loadBoardsStore()
		if err != nil {
			return err
		}
		watched := make(map[string]bool, len(bf.Boards))
		for _, b := range bf.Boards {
			watched[strings.TrimSuffix(strings.ToLower(b.URL), "/")] = true
		}

		facets := discovery.HardcodedFacets()
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

		// Read back this run's rows so output shows true post-save
		// status (a re-run over a dismissed company reports dismissed).
		persisted, err := store.Candidates("")
		if err != nil {
			return formatError("list candidates", err)
		}
		statusByName := make(map[string]string, len(persisted))
		for _, c := range persisted {
			statusByName[c.Name] = c.Status
		}

		type row struct {
			Name   string              `json:"name"`
			Domain string              `json:"domain"`
			Facet  string              `json:"facet"`
			Boards []db.CandidateBoard `json:"boards"`
			Status string              `json:"status"`
		}
		out := make([]row, 0, len(found))
		for _, c := range found {
			r := row{Name: c.Name, Domain: c.Domain, Facet: c.Facet, Status: statusByName[c.Name]}
			if bs, ok := lookupCandidateBoards(persisted, c.Name); ok {
				r.Boards = bs
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
			rows = append(rows, []string{r.Name, r.Facet, strings.Join(shorts, ", "), r.Status})
		}
		fmt.Println(formatTable([]string{"Company", "Facet", "Boards", "Status"}, rows))
		fmt.Printf("\n  %d company(ies) discovered. Add or dismiss with 'waypoint discover add|dismiss'.\n\n", len(out))
		return nil
	},
}

// lookupCandidateBoards finds one candidate's persisted boards by name.
func lookupCandidateBoards(cands []db.CompanyCandidate, name string) ([]db.CandidateBoard, bool) {
	for _, c := range cands {
		if c.Name == name {
			return c.Boards, true
		}
	}
	return nil, false
}

func init() {
	discoverCmd.AddCommand(discoverRunCmd)
	rootCmd.AddCommand(discoverCmd)
}
