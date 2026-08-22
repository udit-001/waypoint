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
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/zen"
)

// runDiscovery is the seam between the command and the discovery
// module: a variable so CLI tests stub the network-touching pipeline.
var runDiscovery = discovery.Discover

// enumerateFacets produces the facet list for one run: brief→zen→Exa
// when both API keys are set, otherwise the validated hardcoded list.
// real reports which path ran; tests stub it.
var enumerateFacets = func(ctx context.Context, store db.Store) ([]discovery.Facet, string, error) {
	settings, err := store.GetSettings()
	if err != nil {
		return nil, "", err
	}
	hasExa := strings.TrimSpace(settings.ExaAPIKey) != ""
	hasZen := strings.TrimSpace(settings.ZenAPIKey) != ""
	if !hasExa || !hasZen {
		return discovery.HardcodedFacets(), "hardcoded", nil
	}

	brief, err := store.GetBrief()
	if err != nil {
		return nil, "", err
	}
	text := briefText(brief)
	hash := discovery.BriefHash(text)

	// Facet cache: an unchanged brief skips the LLM expansion.
	if cached, ok, _ := store.DiscoveryFacets(hash); ok && len(cached) > 0 {
		return toFacets(cached), "cached", nil
	}

	zcfg := zen.DefaultConfig()
	zcfg.APIKey = zen.ResolveKey(settings.ZenAPIKey)
	zenClient := zen.New(zcfg)
	facets, err := discovery.ExpandFacets(ctx, text, zenClient, maxExpandFacets)
	if err != nil {
		return nil, "", err
	}
	if err := store.SaveDiscoveryFacets(hash, facets); err != nil {
		return nil, "", err
	}

	var headers map[string]string
	if hasExa {
		headers = map[string]string{"x-api-key": strings.TrimSpace(settings.ExaAPIKey)}
	}
	enum := &exaEnumerator{c: exa.New("", headers)}
	out, err := discovery.Enumerate(ctx, facets, enum, maxEnumerateCalls)
	if err != nil {
		return nil, "", err
	}
	return out, "brief+exa", nil
}

const (
	maxExpandFacets   = 12 // LLM expansion cap
	maxEnumerateCalls = 60 // Exa-call bound per run (~30-60 budget window)
)

// exaEnumerator adapts the Exa client to the discovery enumerator seam.
type exaEnumerator struct{ c *exa.Client }

func (e *exaEnumerator) Companies(ctx context.Context, facet string) ([]discovery.Company, error) {
	hits, err := e.c.SearchCompanies(ctx, facet, 15)
	if err != nil {
		return nil, err
	}
	out := make([]discovery.Company, 0, len(hits))
	for _, h := range hits {
		domain := domainOf(h.URL)
		if h.Name == "" || domain == "" {
			continue
		}
		out = append(out, discovery.Company{Name: h.Name, Domain: domain})
	}
	return out, nil
}

// domainOf extracts the hostname minus www. from a URL; empty on garbage.
func domainOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// briefText renders the curation brief as compact text for the LLM.
func briefText(b db.Brief) string {
	var sb strings.Builder
	if b.Facts.Title != "" {
		sb.WriteString("title: " + b.Facts.Title + "\n")
	}
	if b.Facts.Seniority != "" {
		sb.WriteString("seniority: " + b.Facts.Seniority + "\n")
	}
	if len(b.Facts.Skills) > 0 {
		sb.WriteString("skills: " + strings.Join(b.Facts.Skills, ", ") + "\n")
	}
	if len(b.Preferences.Keywords) > 0 {
		sb.WriteString("interests: " + strings.Join(b.Preferences.Keywords, ", ") + "\n")
	}
	if len(b.Preferences.Companies) > 0 {
		sb.WriteString("companies liked: " + strings.Join(b.Preferences.Companies, ", ") + "\n")
	}
	if b.Preferences.Remote != "" {
		sb.WriteString("remote: " + b.Preferences.Remote + "\n")
	}
	if len(b.Open) > 0 {
		sb.WriteString("open questions: " + strings.Join(b.Open, "; ") + "\n")
	}
	return sb.String()
}

// toFacets wraps plain labels in the pipeline's facet shape.
func toFacets(labels []string) []discovery.Facet {
	out := make([]discovery.Facet, 0, len(labels))
	for _, l := range labels {
		out = append(out, discovery.Facet{Name: l})
	}
	return out
}

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

		facets, source, err := enumerateFacets(cmd.Context(), store)
		if err != nil {
			return formatError("discover", err)
		}
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
				"meta":       map[string]any{"facets": len(facets), "probed": len(found), "found": len(out), "source": source},
				"candidates": out,
			})
			return nil
		}

		fmt.Println()
		if source != "hardcoded" {
			fmt.Printf("  Facets via %s (%d facet(s)).\n\n", source, len(facets))
		} else {
			fmt.Println("  Using the built-in starter facets. Set zen_api_key and exa_api_key (waypoint settings) to enumerate from your brief instead.")
			fmt.Println()
		}
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
