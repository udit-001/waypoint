package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/linkedin"
)

// importLinkedInTimeout bounds the Exa fetch. The MCP client itself times out
// at 50s; the server WriteTimeout (60s) accommodates the whole request.
const importLinkedInTimeout = 50 * time.Second

// handleImportLinkedIn fetches a public LinkedIn profile through Exa MCP and
// merges it into the stored profile. It never writes: the response carries the
// merged profile document (the same camelCase keys PATCH /api/profile accepts)
// plus a MergeSummary diff, and the web UI previews both before PATCHing on
// Apply — the existing document-validation path stays the only writer.
//
// The merge is unconditional: an empty stored profile merges to the fetched
// profile (every entry "added"), so the seed and update flows share one shape.
//
// Errors are classified for the UI: a malformed URL is a 400 (client fix),
// an unreachable/blocked profile is a 502 (transient — retry), and a fetched
// page with nothing parseable is a 422 (LinkedIn served a login wall).
func handleImportLinkedIn(store db.Store, f *linkedin.Fetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if _, err := linkedin.ValidateURL(body.URL); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), importLinkedInTimeout)
		defer cancel()

		fetched, err := f.FetchProfile(ctx, body.URL)
		if err != nil {
			jsonError(w, "couldn't fetch this LinkedIn profile — Exa may be rate-limited or LinkedIn may be blocking it. Try again in a moment.",
				http.StatusBadGateway)
			return
		}
		if fetched.Empty() {
			jsonError(w, "couldn't find profile details — LinkedIn may be blocking this profile. Make sure it's public, then try again.",
				http.StatusUnprocessableEntity)
			return
		}

		current, err := store.GetProfile()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		merged, summary := linkedin.Merge(profileToLinkedIn(current), fetched)

		jsonResponse(w, map[string]any{
			"doc":     merged.Doc(),
			"summary": summary,
		})
	}
}

// profileToLinkedIn adapts the stored profile to the linkedin-domain shape the
// merge operates on. The importable fields only — the merge never touches
// email, phone, preferences, or constraints.
func profileToLinkedIn(p db.Profile) linkedin.Profile {
	var skills []string
	json.Unmarshal([]byte(p.Skills), &skills)

	out := linkedin.Profile{
		Name:     p.Name,
		Headline: p.Title,
		Location: p.CurrentLocation,
		Skills:   skills,
	}
	for _, e := range db.ParseExperienceEntries(p.Experience) {
		out.Exp = append(out.Exp, linkedin.Experience{
			Title:       e.Title,
			Company:     e.Company,
			Start:       e.Start,
			End:         e.End,
			Description: e.Description,
		})
	}
	for _, e := range db.ParseEducationEntries(p.Education) {
		out.Edu = append(out.Edu, linkedin.Education{
			Institution: e.Institution,
			Degree:      e.Degree,
			Start:       e.Start,
			End:         e.End,
			Description: e.Description,
		})
	}
	return out
}
