package db

import (
	"encoding/json"
	"fmt"
)

// Candidate statuses — the discovery ledger's review vocabulary
// (add/dismiss lands with WP-151).
const (
	StatusCandidateSuggested = "suggested"
	StatusCandidateAdded     = "added"
	StatusCandidateDismissed = "dismissed"
)

// CandidateBoard is one ATS board link discovered for a candidate
// company — the same shape boards.toml entries grow from.
type CandidateBoard struct {
	Provider string `json:"provider"` // greenhouse | lever | ashby | workday | eightfold
	URL      string `json:"url"`
}

// CompanyCandidate is one row of the discovery ledger: a company the
// pipeline surfaced, its verified ATS boards, and where it stands in
// review. Durable across runs: re-discovery upserts by name without
// resetting review status.
type CompanyCandidate struct {
	ID        int64            `db:"id" json:"id"`
	Name      string           `db:"name" json:"name"`
	Domain    string           `db:"domain" json:"domain"`
	Boards    []CandidateBoard `db:"-" json:"boards"`
	Facet     string           `db:"facet" json:"facet"`
	Status    string           `db:"status" json:"status"`
	CreatedAt string           `db:"created_at" json:"createdAt"`
}

const candidateColumns = `id, name, domain, boards, facet, status, created_at`

func marshalCandidateBoards(boards []CandidateBoard) (string, error) {
	if len(boards) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(boards)
	if err != nil {
		return "", fmt.Errorf("marshal candidate boards: %w", err)
	}
	return string(raw), nil
}

func scanCandidate(row interface{ Scan(...any) error }) (CompanyCandidate, error) {
	var (
		c         CompanyCandidate
		boardsRaw string
	)
	err := row.Scan(&c.ID, &c.Name, &c.Domain, &boardsRaw, &c.Facet, &c.Status, &c.CreatedAt)
	if err != nil {
		return CompanyCandidate{}, err
	}
	c.Boards = []CandidateBoard{}
	if boardsRaw != "" && boardsRaw != "[]" {
		if err := json.Unmarshal([]byte(boardsRaw), &c.Boards); err != nil {
			return CompanyCandidate{}, fmt.Errorf("unmarshal candidate boards: %w", err)
		}
	}
	return c, nil
}

// SaveCandidates upserts discovered companies by unique name. The deep
// behavior is the re-run contract:
//
//   - new rows insert as "suggested";
//   - rows already added or dismissed keep their status AND their
//     original boards — re-discovery never resets a review decision;
//   - rows still suggested get their domain/boards/facet refreshed, so
//     a later run that finds more boards updates the suggestion.
func (s *SQLiteStore) SaveCandidates(cands []CompanyCandidate) error {
	for _, c := range cands {
		boardsJSON, err := marshalCandidateBoards(c.Boards)
		if err != nil {
			return err
		}
		if _, err := s.Exec(
			`INSERT INTO company_candidates (name, domain, boards, facet)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(name) DO UPDATE SET
			     domain = excluded.domain,
			     boards = excluded.boards,
			     facet  = excluded.facet
			 WHERE company_candidates.status = 'suggested'`,
			c.Name, c.Domain, boardsJSON, c.Facet,
		); err != nil {
			return fmt.Errorf("save candidate %q: %w", c.Name, err)
		}
	}
	return nil
}

// Candidates returns discovery candidates, optionally filtered by status
// ("suggested", "added", "dismissed"; "" = all), oldest first.
func (s *SQLiteStore) Candidates(status string) ([]CompanyCandidate, error) {
	query := fmt.Sprintf("SELECT %s FROM company_candidates", candidateColumns)
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY id"

	rows, err := s.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CompanyCandidate
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCandidateStatus moves one candidate through the review vocabulary.
// Errors when the id is unknown.
func (s *SQLiteStore) SetCandidateStatus(id int64, status string) error {
	result, err := s.Exec("UPDATE company_candidates SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("set candidate status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no candidate with id %d", id)
	}
	return nil
}

var _ Store = (*SQLiteStore)(nil)
