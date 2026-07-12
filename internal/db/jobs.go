package db

import (
	"fmt"
	"strings"
	"time"
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
func (s *Store) GetJobs() ([]Job, error) {
	rows, err := s.Query(fmt.Sprintf("SELECT %s %s ORDER BY j.id DESC", jobColumns, jobFrom))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// GetJob returns a single job by ID.
func (s *Store) GetJob(id int64) (Job, error) {
	row := s.QueryRow(fmt.Sprintf("SELECT %s %s WHERE j.id = ?", jobColumns, jobFrom), id)
	return scanJob(row)
}

// AddJob creates a new job.
func (s *Store) AddJob(j Job) (Job, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if j.Status == "" {
		j.Status = "Not Applied"
	}

	var categoryID any
	if j.CategoryID != 0 {
		categoryID = j.CategoryID
	}

	result, err := s.Exec(
		`INSERT INTO jobs (company, position, date, applied_date, status, category_id, salary, location, contact, url, notes, reminder_date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.Company, j.Position, j.Date, j.AppliedDate, j.Status, categoryID,
		j.Salary, j.Location, j.Contact, j.URL, j.Notes, j.ReminderDate,
		now, now,
	)
	if err != nil {
		return Job{}, fmt.Errorf("add job: %w", err)
	}

	id, _ := result.LastInsertId()
	j.ID = id
	j.CreatedAt = now
	j.UpdatedAt = now

	// Record history
	if err := s.AddHistory(id, "Created", "", j.Status); err != nil {
		return Job{}, fmt.Errorf("add history: %w", err)
	}

	return s.GetJob(id)
}

// UpdateJob updates fields of an existing job. Only non-zero fields are applied.
func (s *Store) UpdateJob(id int64, updates map[string]any) (Job, error) {
	if len(updates) == 0 {
		return s.GetJob(id)
	}

	// Fetch the old job for history tracking
	oldJob, err := s.GetJob(id)
	if err != nil {
		return Job{}, fmt.Errorf("get job for update: %w", err)
	}

	// Build SET clause
	var setClauses []string
	var args []any

	columnMap := map[string]string{
		"company":      "company",
		"position":     "position",
		"date":         "date",
		"applied_date": "applied_date",
		"status":       "status",
		"category_id":  "category_id",
		"salary":       "salary",
		"location":     "location",
		"contact":      "contact",
		"url":          "url",
		"notes":        "notes",
		"reminder_date": "reminder_date",
	}

	// Track status change for history
	var oldStatus, newStatus string
	var statusChanged bool

	for key, col := range columnMap {
		if val, ok := updates[key]; ok {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)

			if key == "status" {
				oldStatus = oldJob.Status
				newStatus = fmt.Sprint(val)
				statusChanged = oldStatus != newStatus
			}
		}
	}

	if len(setClauses) == 0 {
		return oldJob, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now)
	args = append(args, id)

	query := fmt.Sprintf("UPDATE jobs SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	if _, err := s.Exec(query, args...); err != nil {
		return Job{}, fmt.Errorf("update job: %w", err)
	}

	// Record history
	if statusChanged {
		if err := s.AddHistory(id, "Status", oldStatus, newStatus); err != nil {
			return Job{}, fmt.Errorf("add status history: %w", err)
		}
	} else {
		if err := s.AddHistory(id, "Updated", "", ""); err != nil {
			return Job{}, fmt.Errorf("add update history: %w", err)
		}
	}

	return s.GetJob(id)
}

// DeleteJob deletes a job by ID. History is automatically cascade-deleted
// by the ON DELETE CASCADE foreign key on history.job_id.
func (s *Store) DeleteJob(id int64) error {
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
func (s *Store) SearchJobs(query string, status, category string) ([]Job, error) {
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
func (s *Store) FilterJobs(status, category string) ([]Job, error) {
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
func (s *Store) JobCount() (int, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM jobs")
	return count, err
}

// JobExists returns true if a job with the given URL is already tracked.
func (s *Store) JobExists(url string) (bool, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM jobs WHERE url = ?", url)
	return count > 0, err
}
