package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/udit-001/waypoint/internal/db"
)

// handleUpdateSettings accepts partial settings updates. Only provided
// fields are changed — missing keys are left untouched. The autopilot
// toggle is gated: enabling requires the brief to be Complete.
func handleUpdateSettings(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&body); err != nil {
			if errors.Is(err, io.EOF) {
				jsonError(w, "no fields provided", http.StatusBadRequest)
				return
			}
			jsonError(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		if len(body) == 0 {
			jsonError(w, "no fields provided", http.StatusBadRequest)
			return
		}

		// Gate: autopilot can only be enabled when brief is Complete.
		if enabled, ok := body["autopilot_enabled"]; ok {
			enableVal, _ := enabled.(float64) // JSON numbers are float64
			if enableVal == 1 {
				brief, err := store.GetBrief()
				if err != nil {
					jsonError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if !brief.Complete {
					jsonError(w, "complete your curation brief before enabling autopilot", http.StatusBadRequest)
					return
				}
			}
		}

		updates := make(map[string]any)
		for k, v := range body {
			switch k {
			case "theme", "default_view", "autopilot_provider", "zen_api_key":
				updates[k] = v
			case "items_per_page", "autopilot_cadence":
				// Accept both float64 (JSON) and int.
				if f, ok := v.(float64); ok {
					updates[k] = int(f)
				} else if i, ok := v.(int); ok {
					updates[k] = i
				}
			case "autopilot_enabled":
				if f, ok := v.(float64); ok {
					updates[k] = int(f)
				} else if i, ok := v.(int); ok {
					updates[k] = i
				}
			default:
				// Ignore unknown keys silently (forward-compatible).
			}
		}

		if len(updates) == 0 {
			jsonError(w, "no valid fields provided", http.StatusBadRequest)
			return
		}

		if err := store.UpsertSettings(updates); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Return updated settings.
		settings, err := store.GetSettings()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, settings)
	}
}
