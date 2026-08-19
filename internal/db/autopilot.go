package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// RunLog records one autopilot cycle run.
type RunLog struct {
	ID                  int64  `json:"id" db:"id"`
	StartedAt           string `json:"startedAt" db:"started_at"`
	FinishedAt          string `json:"finishedAt,omitempty" db:"finished_at"`
	DurationMs          int64  `json:"durationMs,omitempty" db:"duration_ms"`
	PostingsNew         int    `json:"postingsNew" db:"postings_new"`
	PostingsShortlisted int    `json:"postingsShortlisted" db:"postings_shortlisted"`
	PostingsDismissed   int    `json:"postingsDismissed" db:"postings_dismissed"`
	PostingsErrored     int    `json:"postingsErrored" db:"postings_errored"`
	Errors              string `json:"errors" db:"errors"` // JSON array of error strings
}

// AddRunLog inserts a new run log entry and returns its ID.
func (s *SQLiteStore) AddRunLog(entry RunLog) (int64, error) {
	if entry.StartedAt == "" {
		entry.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if entry.Errors == "" {
		entry.Errors = "[]"
	}
	result, err := s.Exec(
		`INSERT INTO autopilot_runs (started_at, finished_at, duration_ms,
			postings_new, postings_shortlisted, postings_dismissed, postings_errored, errors)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.StartedAt, entry.FinishedAt, entry.DurationMs,
		entry.PostingsNew, entry.PostingsShortlisted, entry.PostingsDismissed,
		entry.PostingsErrored, entry.Errors,
	)
	if err != nil {
		return 0, fmt.Errorf("insert run log: %w", err)
	}
	return result.LastInsertId()
}

// UpdateRunLog updates an existing run log entry by ID.
func (s *SQLiteStore) UpdateRunLog(id int64, entry RunLog) error {
	_, err := s.Exec(
		`UPDATE autopilot_runs SET
			finished_at = ?, duration_ms = ?,
			postings_new = ?, postings_shortlisted = ?,
			postings_dismissed = ?, postings_errored = ?,
			errors = ?
		 WHERE id = ?`,
		entry.FinishedAt, entry.DurationMs,
		entry.PostingsNew, entry.PostingsShortlisted,
		entry.PostingsDismissed, entry.PostingsErrored,
		entry.Errors, id,
	)
	if err != nil {
		return fmt.Errorf("update run log: %w", err)
	}
	return nil
}

// GetLastRun returns the most recent run log entry.
func (s *SQLiteStore) GetLastRun() (RunLog, bool, error) {
	var entry RunLog
	err := s.Get(&entry,
		`SELECT id, started_at, finished_at, duration_ms,
			postings_new, postings_shortlisted, postings_dismissed, postings_errored, errors
		 FROM autopilot_runs ORDER BY id DESC LIMIT 1`)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return RunLog{}, false, nil
		}
		return RunLog{}, false, err
	}
	return entry, true, nil
}

// --- FakeStore ---

// AddRunLog inserts a run log entry into the fake store.
func (f *FakeStore) AddRunLog(entry RunLog) (int64, error) {
	f.nextRunLogID++
	entry.ID = f.nextRunLogID
	if entry.StartedAt == "" {
		entry.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if entry.Errors == "" {
		entry.Errors = "[]"
	}
	f.RunLogs = append(f.RunLogs, entry)
	return entry.ID, nil
}

// UpdateRunLog updates a run log entry in the fake store.
func (f *FakeStore) UpdateRunLog(id int64, entry RunLog) error {
	for i, r := range f.RunLogs {
		if r.ID == id {
			entry.ID = id
			f.RunLogs[i] = entry
			return nil
		}
	}
	return fmt.Errorf("run log %d not found", id)
}

// GetLastRun returns the most recent run log entry from the fake store.
func (f *FakeStore) GetLastRun() (RunLog, bool, error) {
	if len(f.RunLogs) == 0 {
		return RunLog{}, false, nil
	}
	last := f.RunLogs[len(f.RunLogs)-1]
	return last, true, nil
}

// ErrorsSlice parses the Errors JSON string into a slice.
func (r RunLog) ErrorsSlice() []string {
	if r.Errors == "" || r.Errors == "[]" {
		return nil
	}
	var errs []string
	if err := json.Unmarshal([]byte(r.Errors), &errs); err != nil {
		return nil
	}
	return errs
}
