package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/sweeper"
)

var errCompanyNotFound = errors.New("company not found")

// webSweepMaxPages caps a web-triggered one-board sweep so an
// interactive click can't run past the server's write timeout. Deeper
// backlogs belong to 'boards sweep' / the autopilot cycle.
const webSweepMaxPages = 10

// handlePauseResumeCompany flips a board's Enabled flag in boards.toml
// through the injected mutator — never a direct TOML write.
func handlePauseResumeCompany(withBoards func(func(*config.BoardsFile) error) error, enable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if withBoards == nil {
			jsonError(w, "boards file is not writable from the web UI", http.StatusServiceUnavailable)
			return
		}
		var found bool
		err := withBoards(func(bf *config.BoardsFile) error {
			e := bf.Find(name)
			if e == nil {
				return errCompanyNotFound
			}
			found = true
			e.Enabled = enable
			return nil
		})
		if err != nil {
			if errors.Is(err, errCompanyNotFound) {
				jsonError(w, "no board named "+name, http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		state := "paused"
		if enable {
			state = "active"
		}
		jsonResponse(w, map[string]any{"meta": map[string]any{"name": name, "updated": found, "state": state}})
	}
}

// handleRemoveCompany deletes a board entry from boards.toml. The client
// keeps the removed entry and offers undo within its toast window.
func handleRemoveCompany(withBoards func(func(*config.BoardsFile) error) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if withBoards == nil {
			jsonError(w, "boards file is not writable from the web UI", http.StatusServiceUnavailable)
			return
		}
		var removed bool
		err := withBoards(func(bf *config.BoardsFile) error {
			removed = bf.Remove(name)
			if !removed {
				return errCompanyNotFound
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, errCompanyNotFound) {
				jsonError(w, "no board named "+name, http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]any{"meta": map[string]any{"name": name, "removed": true}})
	}
}

// handleRestoreCompany upserts a full board entry — the undo path for
// Remove. The body is the same shape boards.toml stores.
func handleRestoreCompany(withBoards func(func(*config.BoardsFile) error) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if withBoards == nil {
			jsonError(w, "boards file is not writable from the web UI", http.StatusServiceUnavailable)
			return
		}
		var entry config.BoardEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			jsonError(w, "invalid board payload", http.StatusBadRequest)
			return
		}
		if entry.Name == "" {
			entry.Name = name
		}
		if entry.Name != name {
			jsonError(w, "payload name does not match URL", http.StatusBadRequest)
			return
		}

		err := withBoards(func(bf *config.BoardsFile) error {
			bf.Upsert(entry)
			return nil
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]any{"meta": map[string]any{"name": name, "restored": true}})
	}
}

// handleSweepCompany runs one board's sweep through the shared sweeper
// core and records the sweep state — the exact contract the trust strip
// reads, so a web-triggered sweep looks identical to a CLI one.
func handleSweepCompany(store db.Store, loadBoards func() ([]config.BoardEntry, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if loadBoards == nil {
			jsonError(w, "boards unavailable", http.StatusServiceUnavailable)
			return
		}
		entries, err := loadBoards()
		if err != nil {
			jsonError(w, "failed to load boards", http.StatusInternalServerError)
			return
		}
		var entry *config.BoardEntry
		for i := range entries {
			if entries[i].Name == name {
				entry = &entries[i]
				break
			}
		}
		if entry == nil {
			jsonError(w, "no board named "+name, http.StatusNotFound)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
		defer cancel()
		// Clamp the board's own page cap: a web-triggered sweep must not
		// run away, and 0/uncapped entries default to the web cap. SweepOne
		// reads MaxPages off the entry.
		if entry.MaxPages <= 0 || entry.MaxPages > webSweepMaxPages {
			entry.MaxPages = webSweepMaxPages
		}
		res, serr := sweeper.SweepOne(ctx, store, *entry, 90, 0)

		sweepErr := ""
		if serr != nil {
			sweepErr = serr.Error()
		}
		if serr2 := store.SetBoardSweepState(name, db.BoardSweepState{
			At: time.Now().UTC().Format(time.RFC3339), OK: serr == nil, Error: sweepErr,
		}); serr2 != nil {
			jsonError(w, "failed to record sweep state", http.StatusInternalServerError)
			return
		}

		if serr != nil {
			jsonResponse(w, map[string]any{"meta": map[string]any{
				"name": name, "failed": true, "error": sweepErr,
			}})
			return
		}
		jsonResponse(w, map[string]any{"meta": map[string]any{
			"name": name, "fetched": res.Fetched, "new": res.New,
			"seen": res.Seen, "failed": false,
		}})
	}
}

// errorsIs is a tiny alias keeping this file free of an errors import
// alongside the stdlib errors used elsewhere in the package.
func errorsIs(err, target error) bool {
	return err == target || (err != nil && target != nil && err.Error() == target.Error())
}
