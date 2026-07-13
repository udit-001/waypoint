package db

import "fmt"

const historyColumns = `id, job_id, action, from_value, to_value, timestamp`

// scanHistory scans a history row.
func scanHistory(row interface{ Scan(...any) error }) (HistoryEntry, error) {
	var h HistoryEntry
	err := row.Scan(&h.ID, &h.JobID, &h.Action, &h.From, &h.To, &h.Timestamp)
	return h, err
}

// AddHistory records a history entry for a job.
func (s *SQLiteStore) AddHistory(jobID int64, action, from, to string) error {
	_, err := s.Exec(
		`INSERT INTO history (job_id, action, from_value, to_value) VALUES (?, ?, ?, ?)`,
		jobID, action, from, to,
	)
	return err
}

// GetJobHistory returns all history entries for a job, newest first.
func (s *SQLiteStore) GetJobHistory(jobID int64) ([]HistoryEntry, error) {
	rows, err := s.Query(
		fmt.Sprintf("SELECT %s FROM history WHERE job_id = ? ORDER BY id DESC", historyColumns),
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryEntry
	for rows.Next() {
		h, err := scanHistory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

// GetAllHistory returns all history entries, newest first.
func (s *SQLiteStore) GetAllHistory() ([]HistoryEntry, error) {
	rows, err := s.Query(fmt.Sprintf("SELECT %s FROM history ORDER BY id DESC LIMIT 500", historyColumns))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []HistoryEntry
	for rows.Next() {
		h, err := scanHistory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}
