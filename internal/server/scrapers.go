package server

import (
	"net/http"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// scraperView is one row of the Sources tab (WP-183): the compile-time
// registry entry joined with the user's opt-out from settings.
//
// Name is the display name (Source()); id is the opt-out key the PATCH
// whitelist writes. Aggregator mirrors the registry's own category —
// LinkedIn/Indeed/Google Jobs carry the chip, portals don't.
type scraperView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Aggregator bool   `json:"aggregator"`
	Disabled   bool   `json:"disabled"`
}

// handleListScrapers serves GET /api/scrapers. The registry is
// compile-time (scraper.All()), so the only per-user fact here is the
// disabled list from settings.
func handleListScrapers(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := store.GetSettings()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		disabled := make(map[string]bool)
		for _, id := range db.ParseDisabledScrapers(settings.AutopilotDisabledScrapers) {
			disabled[id] = true
		}

		all := scraper.All()
		out := make([]scraperView, 0, len(all))
		for _, s := range all {
			agg := false
			for _, c := range s.Categories() {
				if c == "aggregator" {
					agg = true
					break
				}
			}
			out = append(out, scraperView{
				ID:         s.Name(),
				Name:       s.Source(),
				Aggregator: agg,
				Disabled:   disabled[s.Name()],
			})
		}
		jsonResponse(w, map[string]any{"scrapers": out})
	}
}
