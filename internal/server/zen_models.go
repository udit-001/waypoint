package server

import (
	"net/http"

	"github.com/udit-001/waypoint/internal/zen"
)

// zenCatalogURL is the test seam for the catalog's upstream.
var zenCatalogURL = zen.FreeModelsCDNURL

// handleZenModels serves the curated free-models list from the catalog
// (stale-while-revalidate over the jsdelivr CDN). The fetch is anonymous —
// no key gate, no hardcoded fallback (decision 2026-09-19: a static list
// goes stale exactly like the one this replaces). When the catalog has no
// last-known-good and the CDN is unreachable, the endpoint answers 502
// and the web dropdown degrades to the saved selection.
func handleZenModels(catalog *zen.Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		models, err := catalog.Models(r.Context())
		if err != nil {
			jsonError(w, "curated model list unavailable: "+err.Error(), http.StatusBadGateway)
			return
		}
		jsonResponse(w, map[string]any{"models": models, "source": "cdn"})
	}
}
