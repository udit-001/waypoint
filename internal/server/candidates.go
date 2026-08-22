package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
)

var errCandidateNotFound = errors.New("candidate not found")

// handleListCandidates serves the discovery ledger: suggested companies
// awaiting review (plus added/dismissed via ?status=). Raw material of
// the Matches discovery band.
func handleListCandidates(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cands, err := store.Candidates(r.URL.Query().Get("status"))
		if err != nil {
			jsonError(w, "failed to list candidates", http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]any{"candidates": cands})
	}
}

// loadSuggestedCandidate fetches one candidate and enforces that it is
// still awaiting review.
func loadSuggestedCandidate(store db.Store, id int64) (db.CompanyCandidate, error) {
	cands, err := store.Candidates("")
	if err != nil {
		return db.CompanyCandidate{}, err
	}
	for _, c := range cands {
		if c.ID == id {
			if c.Status != db.StatusCandidateSuggested {
				return db.CompanyCandidate{}, fmt.Errorf("candidate %q is already %s", c.Name, c.Status)
			}
			return c, nil
		}
	}
	return db.CompanyCandidate{}, errCandidateNotFound
}

// httpCandidateErr maps promotion failures onto HTTP semantics without
// coupling discovery to HTTP: 404 unknown id, 409 decided/no-op/conflict,
// 502 when the verify gate can't confirm a live board, 500 otherwise.
func httpCandidateErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	switch {
	case errors.Is(err, errCandidateNotFound):
		jsonError(w, "no such candidate", http.StatusNotFound)
	case strings.Contains(msg, "verification failed"),
		strings.Contains(msg, "no provider matched"):
		jsonError(w, msg, http.StatusBadGateway)
	case strings.Contains(msg, "already"),
		strings.Contains(msg, "never replaced"),
		strings.Contains(msg, "no boards to promote"):
		jsonError(w, msg, http.StatusConflict)
	default:
		jsonError(w, msg, http.StatusInternalServerError)
	}
}

// handleAddCandidate promotes a suggested company through the same
// discovery.Promote path as 'discover add' — verify gate, boards.toml
// write via the injected mutator, status flip — and announces it on the
// change bus so open Matches tabs drop the row live.
func handleAddCandidate(store db.Store, withBoards func(func(*config.BoardsFile) error) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			jsonError(w, "invalid candidate id", http.StatusBadRequest)
			return
		}

		cand, err := loadSuggestedCandidate(store, id)
		if err != nil {
			httpCandidateErr(w, err)
			return
		}
		if withBoards == nil {
			jsonError(w, "boards file is not writable from the web UI", http.StatusServiceUnavailable)
			return
		}

		links := make([]discovery.BoardLink, 0, len(cand.Boards))
		for _, b := range cand.Boards {
			links = append(links, discovery.BoardLink{Provider: b.Provider, URL: b.URL})
		}

		var out discovery.PromoteOutcome
		perr := withBoards(func(bf *config.BoardsFile) error {
			var perr error
			out, perr = discovery.Promote(r.Context(), cand.Name, links, bf)
			return perr
		})
		if perr != nil {
			httpCandidateErr(w, perr)
			return
		}
		if out.Added == 0 {
			jsonError(w, "board(s) already in boards.toml — nothing to add", http.StatusConflict)
			return
		}

		if serr := store.SetCandidateStatus(id, db.StatusCandidateAdded); serr != nil {
			jsonError(w, "failed to update candidate", http.StatusInternalServerError)
			return
		}
		_ = store.AddChangeEvent("candidates")

		detail := fmt.Sprintf("%d board(s) added via %s, %d jobs on first page", out.Added, out.Provider, out.Fetched)
		if out.AlreadyWatched > 0 {
			detail += fmt.Sprintf(", %d already watched", out.AlreadyWatched)
		}
		jsonResponse(w, map[string]any{"meta": map[string]any{
			"id": id, "name": cand.Name, "updated": true,
			"status": db.StatusCandidateAdded, "detail": detail,
		}})
	}
}

// handleDismissCandidate tombstones a suggested company — the same
// durable decision 'discover dismiss' makes.
func handleDismissCandidate(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			jsonError(w, "invalid candidate id", http.StatusBadRequest)
			return
		}

		cand, cerr := loadSuggestedCandidate(store, id)
		if cerr != nil {
			httpCandidateErr(w, cerr)
			return
		}

		if serr := store.SetCandidateStatus(id, db.StatusCandidateDismissed); serr != nil {
			jsonError(w, "failed to update candidate", http.StatusInternalServerError)
			return
		}
		_ = store.AddChangeEvent("candidates")

		jsonResponse(w, map[string]any{"meta": map[string]any{
			"id": id, "name": cand.Name, "updated": true,
			"status": db.StatusCandidateDismissed,
		}})
	}
}
