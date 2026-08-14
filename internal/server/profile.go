package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/udit-001/waypoint/internal/db"
)

// briefScalarKeys are brief fields stored as a single string.
var briefScalarKeys = map[string]bool{
	"title":            true,
	"seniority":        true,
	"current_location": true,
	"visa_sponsorship": true,
	"remote":           true,
}

// briefListKeys are brief fields stored as a JSON-array string. The Store
// seam normalizes them to the match form (case-fold, trim, dedupe).
var briefListKeys = map[string]bool{
	"skills":              true,
	"location_preference": true,
	"companies":           true,
	"avoid_companies":     true,
	"keywords":            true,
	"dealbreakers":        true,
}

// handleGetBrief serves the curation brief readout — the same shape the agent
// reads via `profile brief --json`. The web Profile view renders it.
func handleGetBrief(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		brief, err := store.GetBrief()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, brief)
	}
}

// handleUpdateProfile is the web's first write route. It accepts the brief
// field keys verbatim (the same keys `profile brief --json` returns), crosses
// the same Store seam as the CLI, and returns the updated brief.
//
// List-valued fields arrive as JSON arrays and are serialized before the seam
// normalizes them. salary_floor arrives as [{region, amount}] — currency is
// never accepted from the client, it is derived from region on read. Unknown
// fields are rejected so a typo never silently drops an edit.
func handleUpdateProfile(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
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

		updates := make(map[string]any)
		for key, raw := range body {
			if !briefScalarKeys[key] && !briefListKeys[key] && key != "salary_floor" {
				jsonError(w, fmt.Sprintf("unknown profile field %q", key), http.StatusBadRequest)
				return
			}

			switch key {
			case "salary_floor":
				floors, err := salaryFloorFromJSON(raw)
				if err != nil {
					jsonError(w, "salary_floor: "+err.Error(), http.StatusBadRequest)
					return
				}
				serialized, err := db.SalaryFloorToJSON(floors)
				if err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
				updates[key] = serialized
			case "seniority":
				// Seniority is a derived fact: once experience carries a year
				// signal, the level is derived, not manually assignable. Manual
				// set is a placeholder for before a resume seed arrives. Mirrors
				// the CLI gate.
				var val string
				if err := json.Unmarshal(raw, &val); err != nil {
					jsonError(w, "seniority: expected a string", http.StatusBadRequest)
					return
				}
				if p, err := store.GetProfile(); err == nil {
					if derived := db.DeriveSeniority(p.Experience); derived != "" {
						jsonError(w, fmt.Sprintf("seniority derives from experience as %q — correct experience instead, or clear it first", derived), http.StatusBadRequest)
						return
					}
				}
				updates[key] = val
			default:
				val, err := briefValueFromJSON(key, raw)
				if err != nil {
					jsonError(w, key+": "+err.Error(), http.StatusBadRequest)
					return
				}
				updates[key] = val
			}
		}

		if err := store.UpsertProfile(updates); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		brief, err := store.GetBrief()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, brief)
	}
}

// briefValueFromJSON validates a scalar/list brief field and returns the value
// ready for the Store seam: scalars stay strings, lists are serialized to the
// JSON-array string the profile columns store.
func briefValueFromJSON(key string, raw json.RawMessage) (string, error) {
	if briefScalarKeys[key] {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", fmt.Errorf("expected a string")
		}
		return s, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return "", fmt.Errorf("expected an array of strings")
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// salaryFloorFromJSON parses a client-provided [{region, amount}] list.
// Currency is a derived fact, never accepted from the client.
func salaryFloorFromJSON(raw json.RawMessage) ([]db.SalaryFloor, error) {
	var entries []struct {
		Region string `json:"region"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("expected an array of {region, amount}")
	}
	floors := make([]db.SalaryFloor, 0, len(entries))
	for _, e := range entries {
		region := strings.TrimSpace(e.Region)
		if region == "" {
			return nil, fmt.Errorf("region is required")
		}
		if e.Amount <= 0 {
			return nil, fmt.Errorf("amount must be a positive number")
		}
		floors = append(floors, db.SalaryFloor{Region: region, Amount: e.Amount})
	}
	return floors, nil
}
