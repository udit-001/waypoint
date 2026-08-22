package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BoardSweepState records how a board's most recent sweep ended: when it
// ran, whether it succeeded, and the error if it didn't. The sweep path
// writes it once per board per sweep; the Companies page renders it as
// the board's trust strip.
type BoardSweepState struct {
	At    string `json:"at"` // RFC3339
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

const kvBoardSweepPrefix = "board_last_swept."

func kvBoardSweepKey(board string) string { return kvBoardSweepPrefix + board }

// SetBoardSweepState upserts the last-sweep record for one board. The
// newest sweep wins: an overwrite replaces the previous record outright.
func (s *SQLiteStore) SetBoardSweepState(board string, st BoardSweepState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal board sweep state: %w", err)
	}
	_, err = s.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		kvBoardSweepKey(board), string(raw),
	)
	if err != nil {
		return fmt.Errorf("save board sweep state: %w", err)
	}
	return nil
}

// GetBoardSweepStates returns every recorded board sweep state, keyed by
// board name. Boards that never swept are simply absent; an untouched
// store yields an empty map.
func (s *SQLiteStore) GetBoardSweepStates() (map[string]BoardSweepState, error) {
	rows, err := s.Query(`SELECT key, value FROM kv WHERE key LIKE ?`, kvBoardSweepPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("load board sweep states: %w", err)
	}
	defer rows.Close()

	out := make(map[string]BoardSweepState)
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, fmt.Errorf("scan board sweep state: %w", err)
		}
		board := strings.TrimPrefix(key, kvBoardSweepPrefix)
		var st BoardSweepState
		if err := json.Unmarshal([]byte(raw), &st); err != nil {
			return nil, fmt.Errorf("unmarshal board sweep state %q: %w", board, err)
		}
		out[board] = st
	}
	return out, rows.Err()
}

// NewPostingCounts counts ledger postings awaiting review ("new") by
// company, matched case-insensitively and keyed lowercase, so callers can
// join them against board display names without agreeing on case.
func (s *SQLiteStore) NewPostingCounts() (map[string]int, error) {
	rows, err := s.Query(
		`SELECT LOWER(company), COUNT(*) FROM postings WHERE status = ? GROUP BY LOWER(company)`,
		StatusNew,
	)
	if err != nil {
		return nil, fmt.Errorf("count new postings: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var company string
		var n int
		if err := rows.Scan(&company, &n); err != nil {
			return nil, fmt.Errorf("scan new posting count: %w", err)
		}
		out[company] = n
	}
	return out, rows.Err()
}

const kvFacetPrefix = "discovery_facets."

// SaveDiscoveryFacets caches the expanded facet list under a brief hash,
// so a repeat discovery run with an unchanged brief skips the LLM
// expansion (WP-152). Newest write wins.
func (s *SQLiteStore) SaveDiscoveryFacets(hash string, facets []string) error {
	raw, err := json.Marshal(facets)
	if err != nil {
		return fmt.Errorf("marshal discovery facets: %w", err)
	}
	_, err = s.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		kvFacetPrefix+hash, string(raw),
	)
	if err != nil {
		return fmt.Errorf("save discovery facets: %w", err)
	}
	return nil
}

// DiscoveryFacets returns the cached facet list for a brief hash.
// found is false when nothing was cached yet.
func (s *SQLiteStore) DiscoveryFacets(hash string) ([]string, bool, error) {
	var raw string
	err := s.Get(&raw, `SELECT value FROM kv WHERE key = ?`, kvFacetPrefix+hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load discovery facets: %w", err)
	}
	var facets []string
	if err := json.Unmarshal([]byte(raw), &facets); err != nil {
		return nil, false, fmt.Errorf("unmarshal discovery facets: %w", err)
	}
	return facets, true, nil
}

const kvDiscoveryLastRun = "discovery_last_run"

// SaveDiscoveryLastRun records when discovery last ran and which brief
// hash it ran against — the trigger state for WP-153.
func (s *SQLiteStore) SaveDiscoveryLastRun(briefHash, atRFC3339 string) error {
	raw, err := json.Marshal(map[string]string{"hash": briefHash, "at": atRFC3339})
	if err != nil {
		return fmt.Errorf("marshal discovery last-run: %w", err)
	}
	_, err = s.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		kvDiscoveryLastRun, string(raw),
	)
	return err
}

const kvAutopilotRunRequest = "autopilot_run_request"

// RequestAutopilotRun drops a one-shot "run now" marker. The daemon's
// scheduler consumes it on its next poll (within seconds), so enabling
// autopilot mid-flight produces matches without waiting a full cadence.
func (s *SQLiteStore) RequestAutopilotRun() error {
	_, err := s.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		kvAutopilotRunRequest, `"now"`,
	)
	if err != nil {
		return fmt.Errorf("request autopilot run: %w", err)
	}
	return nil
}

// ConsumeAutopilotRunRequest reads and clears the run-request marker.
// The SELECT and DELETE are two statements — atomic enough because the
// daemon's scheduler is the single consumer and strictly serial; if a
// second consumer ever appears, wrap this in a transaction.
func (s *SQLiteStore) ConsumeAutopilotRunRequest() (bool, error) {
	var raw string
	err := s.Get(&raw, `SELECT value FROM kv WHERE key = ?`, kvAutopilotRunRequest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("read autopilot run request: %w", err)
	}
	if _, err := s.Exec(`DELETE FROM kv WHERE key = ?`, kvAutopilotRunRequest); err != nil {
		return false, fmt.Errorf("clear autopilot run request: %w", err)
	}
	return true, nil
}

// DiscoveryLastRun returns the persisted trigger state; has is false
// before the first discovery run ever completed.
func (s *SQLiteStore) DiscoveryLastRun() (string, string, bool, error) {
	var raw string
	err := s.Get(&raw, `SELECT value FROM kv WHERE key = ?`, kvDiscoveryLastRun)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	var st struct {
		Hash string `json:"hash"`
		At   string `json:"at"`
	}
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return "", "", false, err
	}
	return st.Hash, st.At, true, nil
}
