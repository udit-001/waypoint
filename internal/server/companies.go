package server

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
)

// CompanyView is one row of the Companies page: a boards.toml entry joined
// with its live stats. Attention-weighted fields first — newCount leads,
// trust strip follows.
type CompanyView struct {
	Name     string `json:"name"`
	Company  string `json:"company"`
	URL      string `json:"url"`
	Provider string `json:"provider,omitempty"`
	Enabled  bool   `json:"enabled"`

	// New postings awaiting review, matched case-insensitively by company.
	NewCount int `json:"newCount"`

	// Trust strip: when this board last swept and how it ended. Stale
	// means last sweep is older than two autopilot cycles.
	LastSweptAt    string `json:"lastSweptAt,omitempty"`
	LastSweepOk    bool   `json:"lastSweepOk"`
	LastSweepError string `json:"lastSweepError,omitempty"`
	Stale          bool   `json:"stale"`
}

// defaultCycleHours mirrors the autopilot cadence default (6h). "A cycle"
// on the Companies page means one autopilot period; two missed cycles
// make a board's freshness stale.
const defaultCycleHours = 6

// handleListCompanies merges boards.toml (read via the injected loader —
// ADR 0001: CLI writes, web reads) with per-company posting counts and
// per-board sweep state from the store.
func handleListCompanies(store db.Store, loadBoards func() ([]config.BoardEntry, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var boards []config.BoardEntry
		if loadBoards != nil {
			var err error
			boards, err = loadBoards()
			if err != nil {
				jsonError(w, "failed to load boards", http.StatusInternalServerError)
				return
			}
		}

		counts, err := store.NewPostingCounts()
		if err != nil {
			jsonError(w, "failed to count postings", http.StatusInternalServerError)
			return
		}
		states, err := store.GetBoardSweepStates()
		if err != nil {
			jsonError(w, "failed to load sweep state", http.StatusInternalServerError)
			return
		}

		cadence := defaultCycleHours
		if settings, err := store.GetSettings(); err == nil && settings.AutopilotCadence > 0 {
			cadence = settings.AutopilotCadence
		}
		staleAfter := time.Duration(2*cadence) * time.Hour

		out := make([]CompanyView, 0, len(boards))
		for _, b := range boards {
			view := CompanyView{
				Name:     b.Name,
				Company:  b.Company,
				URL:      b.URL,
				Provider: b.Provider,
				Enabled:  b.Enabled,
				NewCount: counts[strings.ToLower(b.Company)],
			}
			if st, ok := states[b.Name]; ok {
				view.LastSweptAt = st.At
				view.LastSweepOk = st.OK
				view.LastSweepError = st.Error
				if at, err := time.Parse(time.RFC3339, st.At); err == nil {
					view.Stale = time.Since(at) > staleAfter
				}
			}
			out = append(out, view)
		}

		sort.Slice(out, func(i, j int) bool {
			newsI, newsJ := out[i].NewCount > 0, out[j].NewCount > 0
			if newsI != newsJ {
				return newsI // news floats on top
			}
			if newsI && newsJ {
				return out[i].NewCount > out[j].NewCount // loudest first
			}
			return strings.ToLower(out[i].Company) < strings.ToLower(out[j].Company)
		})

		jsonResponse(w, map[string]any{"companies": out})
	}
}
