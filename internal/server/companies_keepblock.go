package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
)

// Keep/block (WP-181) — the two human actions on an auto-added board.
// Keep = do nothing, made durable: the discovered flag clears and the
// row reads like any hand-added board. Block = remove + never
// re-suggest: every board of the company leaves boards.toml, its domain
// joins the blocklist, and the ledger row tombstones.

// handleKeepCompany clears the Discovered flag on one board entry.
// Idempotent: keeping an already-kept board succeeds.
func handleKeepCompany(withBoards func(func(*config.BoardsFile) error) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if withBoards == nil {
			jsonError(w, "boards file is not writable from the web UI", http.StatusServiceUnavailable)
			return
		}
		err := withBoards(func(bf *config.BoardsFile) error {
			e := bf.Find(name)
			if e == nil {
				return errCompanyNotFound
			}
			e.Discovered = false
			return nil
		})
		if err != nil {
			if err == errCompanyNotFound {
				jsonError(w, "no board named "+name, http.StatusNotFound)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]any{"meta": map[string]any{"name": name, "kept": true}})
	}
}

// handleBlockCompany removes every board entry of the company, persists
// its domain on the discovery blocklist, and tombstones the ledger row.
// The domain comes from the entry itself (auto-adds record it); a manual
// board carries none, so blocking it is rejected rather than half-applied.
func handleBlockCompany(store db.Store, withBoards func(func(*config.BoardsFile) error) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if withBoards == nil {
			jsonError(w, "boards file is not writable from the web UI", http.StatusServiceUnavailable)
			return
		}

		var domain string
		var removed int
		err := withBoards(func(bf *config.BoardsFile) error {
			target := bf.Find(name)
			if target == nil {
				return errCompanyNotFound
			}
			domain = target.Domain
			if strings.TrimSpace(domain) == "" {
				return errBlockNoDomain
			}
			// The entry name is the company slug, so one entry per
			// company today — but removal sweeps by display name so a
			// future multi-board company blocks whole.
			company := strings.ToLower(target.Company)
			kept := bf.Boards[:0]
			for _, e := range bf.Boards {
				if strings.ToLower(e.Company) == company || e.Name == name {
					removed++
					continue
				}
				kept = append(kept, e)
			}
			bf.Boards = kept
			return nil
		})
		if err != nil {
			if err == errCompanyNotFound {
				jsonError(w, "no board named "+name, http.StatusNotFound)
				return
			}
			if err == errBlockNoDomain {
				jsonError(w, "board has no recorded domain — remove it instead", http.StatusConflict)
				return
			}
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := store.BlockDomain(domain); err != nil {
			jsonError(w, "failed to persist blocklist: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := dismissCandidatesByName(store, name); err != nil {
			jsonError(w, "failed to update discovery ledger: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_ = store.AddChangeEvent("companies")

		jsonResponse(w, map[string]any{"meta": map[string]any{
			"name": name, "domain": domain, "removed": removed, "blocked": true,
		}})
	}
}

// errBlockNoDomain marks a board that cannot be blocklisted because no
// domain was recorded with it (a manual add carries no domain).
var errBlockNoDomain = errors.New("board has no recorded domain")

// dismissCandidatesByName tombstones every ledger row for a company name,
// whatever its current review status.
func dismissCandidatesByName(store db.Store, name string) error {
	cands, err := store.Candidates("")
	if err != nil {
		return err
	}
	for _, c := range cands {
		if strings.EqualFold(c.Name, name) && c.Status != db.StatusCandidateDismissed {
			if err := store.SetCandidateStatus(c.ID, db.StatusCandidateDismissed); err != nil {
				return err
			}
		}
	}
	return nil
}
