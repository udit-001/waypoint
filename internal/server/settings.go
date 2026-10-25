package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/udit-001/waypoint/internal/autopilot"
	"github.com/udit-001/waypoint/internal/db"
)

// handleUpdateSettings accepts partial settings updates. Only provided
// fields are changed — missing keys are left untouched. The autopilot
// toggle is gated: enabling requires the brief to be Complete.
// cancelAutopilot (nil-safe) stops the daemon's in-flight cycle when
// autopilot flips off — disable takes effect within a stage boundary,
// not after the full sweep.
func handleUpdateSettings(store db.Store, cancelAutopilot func()) http.HandlerFunc {
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

		// Gate: autopilot can only be enabled when brief is Complete. Same
		// gate the CLI's 'autopilot enable' runs — one contract (WP-158).
		if enabled, ok := body["autopilot_enabled"]; ok {
			enableVal, _ := enabled.(float64) // JSON numbers are float64
			if enableVal == 1 {
				if err := autopilot.CanEnable(r.Context(), store); err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}

		updates := make(map[string]any)
		enabledNow := false
		windowPatched := false
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
			case "items_per_page", "autopilot_cadence", "discovery_interval_days", "discovery_enabled":
				// Accept both float64 (JSON) and int.
				if f, ok := v.(float64); ok {
					updates[k] = int(f)
				} else if i, ok := v.(int); ok {
					updates[k] = i
				}
			case "autopilot_work_start", "autopilot_work_end", "autopilot_working_cadence":
				// Minutes from midnight (window) or minutes (cadence).
				n, ok := 0, false
				if f, isF := v.(float64); isF {
					n, ok = int(f), true
				} else if i, isI := v.(int); isI {
					n, ok = i, true
				}
				if !ok || n < 0 || n > 1439 {
					jsonError(w, k+" must be an integer between 0 and 1439", http.StatusBadRequest)
					return
				}
				updates[k] = n
				windowPatched = true
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

		// The work window is a pair: end must be after start, except both
		// zero which means "reset to the default window". One patch may
		// touch only one side, so validate against the stored other side.
		if windowPatched {
			settings, err := store.GetSettings()
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			start, hasStart := updates["autopilot_work_start"].(int)
			if !hasStart {
				start = settings.AutopilotWorkStart
			}
			end, hasEnd := updates["autopilot_work_end"].(int)
			if !hasEnd {
				end = settings.AutopilotWorkEnd
			}
			if !(end > start || (start == 0 && end == 0)) {
				jsonError(w, "work window: end must be after start (or both 0 for the default)", http.StatusBadRequest)
				return
			}
			if start == 0 && end == 0 {
				updates["autopilot_work_start"] = autopilot.DefaultWorkWindow.Start
				updates["autopilot_work_end"] = autopilot.DefaultWorkWindow.End
			}
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
		} else if _, disabling := updates["autopilot_enabled"]; disabling && cancelAutopilot != nil {
			// Disabling stops the cycle mid-flight: the stages check the
			// context at every boundary, and the interrupted run closes as
			// degraded — recovery semantics for an off-switch, not a crash.
			cancelAutopilot()
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
