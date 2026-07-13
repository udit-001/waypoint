package db

import (
	"fmt"
	"strings"
)

const jobColumns = `j.id, j.company, j.position, j.date, j.applied_date, j.status, COALESCE(j.category_id, 0), COALESCE(c.name, ''), j.salary, j.location, j.contact, j.url, j.notes, j.reminder_date, j.created_at, j.updated_at`

// scanJob scans a single job row from a Row (includes JOIN on categories).
func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var j Job
	err := row.Scan(
		&j.ID, &j.Company, &j.Position, &j.Date, &j.AppliedDate,
		&j.Status, &j.CategoryID, &j.CategoryName, &j.Salary, &j.Location, &j.Contact,
		&j.URL, &j.Notes, &j.ReminderDate, &j.CreatedAt, &j.UpdatedAt,
	)
	return j, err
}

// scanJobs scans job rows.
func scanJobs(rows interface{ Next() bool; Scan(...any) error; Close() error; Err() error }) ([]Job, error) {
	var jobs []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

const jobFrom = `FROM jobs j LEFT JOIN categories c ON j.category_id = c.id`

// GetJobs returns all jobs, sorted by newest first.
func (s *SQLiteStore) GetJobs() ([]Job, error) {
	rows, err := s.Query(fmt.Sprintf("SELECT %s %s ORDER BY j.id DESC", jobColumns, jobFrom))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// GetJob returns a single job by ID.
func (s *SQLiteStore) GetJob(id int64) (Job, error) {
	row := s.QueryRow(fmt.Sprintf("SELECT %s %s WHERE j.id = ?", jobColumns, jobFrom), id)
	return scanJob(row)
}

// InsertJob inserts a job row as-is. No defaults, no history —
// use IntakeAddJob for the full write workflow.
func (s *SQLiteStore) InsertJob(j Job) (Job, error) {
	var categoryID any
	if j.CategoryID != 0 {
		categoryID = j.CategoryID
	}

	result, err := s.Exec(
		`INSERT INTO jobs (company, position, date, applied_date, status, category_id, salary, location, contact, url, notes, reminder_date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.Company, j.Position, j.Date, j.AppliedDate, j.Status, categoryID,
		j.Salary, j.Location, j.Contact, j.URL, j.Notes, j.ReminderDate,
		j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		return Job{}, fmt.Errorf("insert job: %w", err)
	}

	id, _ := result.LastInsertId()
	j.ID = id
	return j, nil
}

// UpdateJobFields executes a raw UPDATE on the jobs table. No history
// recording, no status tracking — use IntakeUpdateJob for the full
// write workflow. Expects "updated_at" in the updates map.
func (s *SQLiteStore) UpdateJobFields(id int64, updates map[string]any) error {
	columnMap := map[string]string{
		"company":       "company",
		"position":      "position",
		"date":          "date",
		"applied_date":  "applied_date",
		"status":        "status",
		"category_id":   "category_id",
		"salary":        "salary",
		"location":      "location",
		"contact":       "contact",
		"url":           "url",
		"notes":         "notes",
		"reminder_date": "reminder_date",
		"updated_at":    "updated_at",
	}

	var setClauses []string
	var args []any
	for key, col := range columnMap {
		if val, ok := updates[key]; ok {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE jobs SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	if _, err := s.Exec(query, args...); err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

// DeleteJob deletes a job by ID. History is automatically cascade-deleted
// by the ON DELETE CASCADE foreign key on history.job_id.
func (s *SQLiteStore) DeleteJob(id int64) error {
	result, err := s.Exec("DELETE FROM jobs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %d not found", id)
	}
	return nil
}

// SearchJobs performs full-text search across company, position, notes, location, contact, and category.
// Optionally filters by status and/or category on top of the FTS results.
func (s *SQLiteStore) SearchJobs(query string, status, category string) ([]Job, error) {
	var conditions []string
	var args []any

	conditions = append(conditions, "jobs_fts MATCH ?")
	args = append(args, query)

	if status != "" {
		conditions = append(conditions, "j.status = ?")
		args = append(args, status)
	}
	if category != "" {
		conditions = append(conditions, "c.name = ?")
		args = append(args, category)
	}

	where := strings.Join(conditions, " AND ")
	rows, err := s.Query(
		fmt.Sprintf("SELECT %s %s JOIN jobs_fts f ON j.id = f.rowid WHERE %s ORDER BY rank", jobColumns, jobFrom, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// FilterJobs returns jobs filtered by status and/or category.
func (s *SQLiteStore) FilterJobs(status, category string) ([]Job, error) {
	var conditions []string
	var args []any

	if status != "" {
		conditions = append(conditions, "j.status = ?")
		args = append(args, status)
	}
	if category != "" {
		conditions = append(conditions, "c.name = ?")
		args = append(args, category)
	}

	query := fmt.Sprintf("SELECT %s %s", jobColumns, jobFrom)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY j.id DESC"

	rows, err := s.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// JobCount returns the total number of jobs.
func (s *SQLiteStore) JobCount() (int, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM jobs")
	return count, err
}

// JobExists returns true if a job with the given URL is already tracked.
func (s *SQLiteStore) JobExists(url string) (bool, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM jobs WHERE url = ?", url)
	return count > 0, err
}
