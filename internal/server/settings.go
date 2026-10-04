package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
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
		enabledNow := false
		for k, v := range body {
			switch k {
			case "theme", "default_view", "autopilot_provider", "zen_api_key", "zen_model", "exa_api_key":
				updates[k] = v
			case "autopilot_disabled_scrapers":
				// The opt-out is a hard constraint on the selection contract:
				// a malformed payload must fail loudly rather than silently
				// leave scrapers gated-in. Stored as the canonical JSON string.
				arr, ok := v.([]any)
				if !ok {
					jsonError(w, "autopilot_disabled_scrapers must be an array of scraper names", http.StatusBadRequest)
					return
				}
				names := make([]string, 0, len(arr))
				for _, item := range arr {
					s, ok := item.(string)
					if !ok {
						jsonError(w, "autopilot_disabled_scrapers must contain only strings", http.StatusBadRequest)
						return
					}
					names = append(names, s)
				}
				updates[k] = db.DisabledScrapersJSON(names)
			case "items_per_page", "autopilot_cadence", "discovery_interval_days":
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
				enabledNow = updates[k] == 1
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

		// Enabling autopilot asks the daemon for an immediate cycle: the
		// scheduler consumes this marker within one poll interval, so
		// matches arrive in seconds — not after a full cadence. Dropped
		// only after the write succeeded (and only on enable).
		if enabledNow {
			if err := store.RequestAutopilotRun(); err != nil {
				log.Printf("settings: request autopilot run: %v", err)
			}
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
