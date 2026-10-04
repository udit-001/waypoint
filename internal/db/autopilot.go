package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// DefaultRunLogLimit is how many runs the runlog surfaces by default —
// the "last ~10 runs" of the human panel and the agent's runs --json.
const DefaultRunLogLimit = 10

// Verdict is a run's outcome: the answer to "why did nothing show up?".
// The vocabulary is shared verbatim across UI, JSON, and docs, so the
// agent's narration matches what the user already saw.
//
// A fifth answer — skipped — is a per-source fact (SourceOutcome.Skipped),
// never a run verdict.
type Verdict string

const (
	// VerdictQuiet: sources ran fine, nothing new fit the brief.
	VerdictQuiet Verdict = "quiet"
	// VerdictNothingSurvived: postings were found, none survived curation.
	VerdictNothingSurvived Verdict = "nothing-survived"
	// VerdictWaitingOnYou: shortlists exist and are unreviewed.
	VerdictWaitingOnYou Verdict = "waiting-on-you"
	// VerdictDegraded: a stage errored, so the run's evidence is partial.
	VerdictDegraded Verdict = "degraded"
)

// IsDry reports whether a verdict counts as a dry cycle: quiet and
// nothing-survived are dry; degraded and waiting-on-you never are. The
// streak is derived from verdict history — no counter is stored.
func (v Verdict) IsDry() bool {
	return v == VerdictQuiet || v == VerdictNothingSurvived
}

// SourceOutcome is one source's row in a run's per-source record.
//
// Scraped and New count this cycle's sweep (results returned, then added
// to the ledger). Shortlisted, Dismissed, and Errored count every posting
// attributed to the source that the cycle processed — including backlog
// postings swept in an earlier cycle. Errored counts failures attributed
// to the source: a posting that failed curation, or the source's own
// fetch failing.
//
// Source is the scraper id. An empty Source means the posting predates
// source recording (a ledger row from before migration 00018).
type SourceOutcome struct {
	Source      string `json:"source"`
	Scraped     int    `json:"scraped"`
	New         int    `json:"new"`
	Shortlisted int    `json:"shortlisted"`
	Dismissed   int    `json:"dismissed"`
	Errored     int    `json:"errored"`
	Skipped     bool   `json:"skipped"`
	Reason      string `json:"reason,omitempty"`
}

// RunLog records one autopilot cycle run.
//
// An unfinished run (FinishedAt empty) is the "running now" signal; its
// Verdict is empty because the run has no outcome yet.
type RunLog struct {
	ID                  int64           `json:"id" db:"id"`
	Verdict             Verdict         `json:"verdict" db:"verdict"`
	StartedAt           string          `json:"startedAt" db:"started_at"`
	FinishedAt          string          `json:"finishedAt,omitempty" db:"finished_at"`
	DurationMs          int64           `json:"durationMs,omitempty" db:"duration_ms"`
	PostingsNew         int             `json:"postingsNew" db:"postings_new"`
	PostingsShortlisted int             `json:"postingsShortlisted" db:"postings_shortlisted"`
	PostingsDismissed   int             `json:"postingsDismissed" db:"postings_dismissed"`
	PostingsErrored     int             `json:"postingsErrored" db:"postings_errored"`
	PerSource           []SourceOutcome `json:"perSource" db:"-"`
	StageErrors         []string        `json:"stageErrors" db:"-"`
}

// runLogColumns selects a full record, coalescing the nullable and
// pre-migration columns so an in-flight or legacy row still scans.
const runLogColumns = `id, COALESCE(verdict, ''), started_at, COALESCE(finished_at, ''),
	COALESCE(duration_ms, 0), postings_new, postings_shortlisted,
	postings_dismissed, postings_errored, COALESCE(per_source, '[]'),
	COALESCE(stage_errors, '[]')`

// scanRunLog reads one runlog row, decoding the JSON columns.
func scanRunLog(row interface{ Scan(...any) error }) (RunLog, error) {
	var (
		entry                     RunLog
		verdict                   string
		perSourceRaw, stageErrRaw string
	)
	if err := row.Scan(
		&entry.ID, &verdict, &entry.StartedAt, &entry.FinishedAt, &entry.DurationMs,
		&entry.PostingsNew, &entry.PostingsShortlisted, &entry.PostingsDismissed,
		&entry.PostingsErrored, &perSourceRaw, &stageErrRaw,
	); err != nil {
		return RunLog{}, err
	}
	entry.Verdict = Verdict(verdict)
	if err := json.Unmarshal([]byte(perSourceRaw), &entry.PerSource); err != nil {
		return RunLog{}, fmt.Errorf("unmarshal per-source record: %w", err)
	}
	if err := json.Unmarshal([]byte(stageErrRaw), &entry.StageErrors); err != nil {
		return RunLog{}, fmt.Errorf("unmarshal stage errors: %w", err)
	}
	return entry, nil
}

// AddRunLog inserts a new run log entry and returns its ID.
func (s *SQLiteStore) AddRunLog(entry RunLog) (int64, error) {
	if entry.StartedAt == "" {
		entry.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	perSource, err := marshalJSONList(entry.PerSource)
	if err != nil {
		return 0, err
	}
	stageErrors, err := marshalJSONList(entry.StageErrors)
	if err != nil {
		return 0, err
	}
	result, err := s.Exec(
		`INSERT INTO autopilot_runs (verdict, started_at, finished_at, duration_ms,
			postings_new, postings_shortlisted, postings_dismissed, postings_errored,
			per_source, stage_errors)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(entry.Verdict), entry.StartedAt, entry.FinishedAt, entry.DurationMs,
		entry.PostingsNew, entry.PostingsShortlisted, entry.PostingsDismissed,
		entry.PostingsErrored, perSource, stageErrors,
	)
	if err != nil {
		return 0, fmt.Errorf("insert run log: %w", err)
	}
	return result.LastInsertId()
}

// UpdateRunLog updates an existing run log entry by ID.
func (s *SQLiteStore) UpdateRunLog(id int64, entry RunLog) error {
	perSource, err := marshalJSONList(entry.PerSource)
	if err != nil {
		return err
	}
	stageErrors, err := marshalJSONList(entry.StageErrors)
	if err != nil {
		return err
	}
	_, err = s.Exec(
		`UPDATE autopilot_runs SET
			verdict = ?, finished_at = ?, duration_ms = ?,
			postings_new = ?, postings_shortlisted = ?,
			postings_dismissed = ?, postings_errored = ?,
			per_source = ?, stage_errors = ?
		 WHERE id = ?`,
		string(entry.Verdict), entry.FinishedAt, entry.DurationMs,
		entry.PostingsNew, entry.PostingsShortlisted,
		entry.PostingsDismissed, entry.PostingsErrored,
		perSource, stageErrors, id,
	)
	if err != nil {
		return fmt.Errorf("update run log: %w", err)
	}
	return nil
}

// GetLastRun returns the most recent run log entry.
func (s *SQLiteStore) GetLastRun() (RunLog, bool, error) {
	row := s.QueryRow(`SELECT ` + runLogColumns + ` FROM autopilot_runs ORDER BY id DESC LIMIT 1`)
	entry, err := scanRunLog(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunLog{}, false, nil
		}
		return RunLog{}, false, err
	}
	return entry, true, nil
}

// ListRunLogs returns the most recent runs, newest first. limit <= 0
// means DefaultRunLogLimit.
func (s *SQLiteStore) ListRunLogs(limit int) ([]RunLog, error) {
	if limit <= 0 {
		limit = DefaultRunLogLimit
	}
	rows, err := s.Query(`SELECT `+runLogColumns+` FROM autopilot_runs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out = make([]RunLog, 0, limit)
	for rows.Next() {
		entry, err := scanRunLog(rows)
		if err != nil {
			return nil, fmt.Errorf("scan run log: %w", err)
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// marshalJSONList encodes a runlog list column, "[]" when empty — the
// same shape whether the list is per-source rows or stage errors.
func marshalJSONList[T any](list []T) (string, error) {
	if len(list) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return "", fmt.Errorf("marshal run log list: %w", err)
	}
	return string(raw), nil
}

// --- FakeStore ---

// AddRunLog inserts a run log entry into the fake store.
func (f *FakeStore) AddRunLog(entry RunLog) (int64, error) {
	f.nextRunLogID++
	entry.ID = f.nextRunLogID
	if entry.StartedAt == "" {
		entry.StartedAt = time.Now().UTC().Format(time.RFC3339)
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

// ListRunLogs returns the most recent run log entries, newest first.
func (f *FakeStore) ListRunLogs(limit int) ([]RunLog, error) {
	if limit <= 0 {
		limit = DefaultRunLogLimit
	}
	out := make([]RunLog, 0, limit)
	for i := len(f.RunLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, f.RunLogs[i])
	}
	return out, nil
}
