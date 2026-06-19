package db

import (
	"fmt"
	"strings"
	"time"
)

const jobColumns = `id, company, position, date, applied_date, status, category, salary, location, contact, url, notes, reminder_date, created_at, updated_at`

// scanJob scans a single job row from a Row.
func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var j Job
	err := row.Scan(
		&j.ID, &j.Company, &j.Position, &j.Date, &j.AppliedDate,
		&j.Status, &j.Category, &j.Salary, &j.Location, &j.Contact,
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

// GetJobs returns all jobs, sorted by newest first.
func (s *Store) GetJobs() ([]Job, error) {
	rows, err := s.Query(fmt.Sprintf("SELECT %s FROM jobs ORDER BY id DESC", jobColumns))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// GetJob returns a single job by ID.
func (s *Store) GetJob(id int64) (Job, error) {
	row := s.QueryRow(fmt.Sprintf("SELECT %s FROM jobs WHERE id = ?", jobColumns), id)
	return scanJob(row)
}

// AddJob creates a new job.
func (s *Store) AddJob(j Job) (Job, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if j.Status == "" {
		j.Status = "Not Applied"
	}
	if j.Category == "" {
		j.Category = "General"
	}

	result, err := s.Exec(
		`INSERT INTO jobs (company, position, date, applied_date, status, category, salary, location, contact, url, notes, reminder_date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.Company, j.Position, j.Date, j.AppliedDate, j.Status, j.Category,
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

	return j, nil
}

// UpdateJob updates fields of an existing job. Only non-zero fields are applied.
// Use UpdateJobFields for more granular control. This method is the primary
// API matching the old JS behavior.
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
		"company":       "company",
		"position":      "position",
		"date":          "date",
		"applied_date":  "applied_date",
		"status":        "status",
		"category":      "category",
		"salary":        "salary",
		"location":      "location",
		"contact":       "contact",
		"url":           "url",
		"notes":         "notes",
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

// DeleteJob deletes a job by ID.
func (s *Store) DeleteJob(id int64) error {
	// Get job for history before deleting
	job, err := s.GetJob(id)
	if err != nil {
		return err
	}

	if err := s.AddHistory(id, "Deleted", "", ""); err != nil {
		return err
	}

	result, err := s.Exec("DELETE FROM jobs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %d not found", id)
	}

	// Clean up history for deleted job
	_, _ = s.Exec("DELETE FROM history WHERE job_id = ?", id)

	_ = job // used for history before delete
	return nil
}

// SearchJobs searches jobs by company, position, or notes.
func (s *Store) SearchJobs(query string) ([]Job, error) {
	like := "%" + query + "%"
	rows, err := s.Query(
		fmt.Sprintf("SELECT %s FROM jobs WHERE company LIKE ? OR position LIKE ? OR notes LIKE ? ORDER BY id DESC", jobColumns),
		like, like, like,
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
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	query := fmt.Sprintf("SELECT %s FROM jobs", jobColumns)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id DESC"

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
