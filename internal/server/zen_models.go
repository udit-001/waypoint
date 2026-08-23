package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/zen"
)

// fallbackFreeModels ships when no API key is stored (the gateway
// requires auth for /v1/models). Refreshed by hand when the free tier
// changes; the live list wins whenever a key exists.
var fallbackFreeModels = []string{
	"x-preview-f-free",
	"mimo-v2.5-free",
	"hy3-free",
	"nemotron-3-ultra-free",
	"nemotron-3.5-lightning-free",
	"laguna-s-2.1-free",
	"deepseek-v4-flash-free",
}

// zenModelsURL is the test seam for the gateway call.
var zenModelsURL = zen.DefaultConfig().BaseURL + "/v1/models"

// handleZenModels lists curation-model choices: the gateway's free-tier
// ids when a key is stored, the static fallback otherwise.
func handleZenModels(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := store.GetSettings()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if key := settings.ZenAPIKey; key != "" {
			client := &http.Client{Timeout: 10 * time.Second}
			req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, zenModelsURL, nil)
			req.Header.Set("Authorization", "Bearer "+key)
			if resp, err := client.Do(req); err == nil {
				defer resp.Body.Close()
				if body, err := io.ReadAll(resp.Body); err == nil && resp.StatusCode == 200 {
					var parsed struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if json.Unmarshal(body, &parsed) == nil {
						free := []string{}
						for _, m := range parsed.Data {
							if strings.HasSuffix(m.ID, "-free") {
								free = append(free, m.ID)
							}
						}
						if len(free) > 0 {
							jsonResponse(w, map[string]any{"models": free, "source": "gateway"})
							return
						}
					}
				}
			}
		}
		jsonResponse(w, map[string]any{"models": fallbackFreeModels, "source": "fallback"})
	}
}
