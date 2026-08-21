package db

import (
	"encoding/json"
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
