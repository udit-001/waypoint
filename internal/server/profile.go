package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/udit-001/waypoint/internal/db"
)

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

// handleUpdateProfile is the web's write route. It accepts the profile
// document — the same camelCase keys `GET /api/profile` and the CLI's
// `profile set --file` use — crosses the same Store seam as the CLI, and
// returns the updated brief. Validation and normalization live in the db
// seam (NormalizeProfileDocument), shared with the CLI, so the two surfaces
// can never drift apart. Unknown keys are rejected so a typo never silently
// drops an edit.
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

		// Current experience gates manual seniority (placeholder until a
		// resume seed arrives) — the same gate the CLI crosses.
		exp, err := store.GetProfile()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		updates, err := db.NormalizeProfileDocument(body, exp)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
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
