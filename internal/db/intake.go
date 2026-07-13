package db

import (
	"fmt"
	"time"
)

// IntakeAddJob creates a new job with defaults, timestamps, and audit history.
// The Store method (InsertJob) does raw CRUD only; this function owns the
// business logic: default status, timestamps, and history recording.
func IntakeAddJob(s Store, j Job) (Job, error) {
	if j.Status == "" {
		j.Status = "Not Applied"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	j.CreatedAt = now
	j.UpdatedAt = now

	j, err := s.InsertJob(j)
	if err != nil {
		return Job{}, fmt.Errorf("add job: %w", err)
	}

	if err := s.AddHistory(j.ID, "Created", "", j.Status); err != nil {
		return Job{}, fmt.Errorf("add history: %w", err)
	}

	return s.GetJob(j.ID)
}

// IntakeUpdateJob updates a job's fields and records audit history.
// The Store method (UpdateJobFields) does raw CRUD only; this function
// owns the business logic: status-change detection and history recording.
func IntakeUpdateJob(s Store, id int64, updates map[string]any) (Job, error) {
	if len(updates) == 0 {
		return s.GetJob(id)
	}

	oldJob, err := s.GetJob(id)
	if err != nil {
		return Job{}, fmt.Errorf("get job for update: %w", err)
	}

	oldStatus := oldJob.Status
	newStatus := ""
	statusChanged := false
	if val, ok := updates["status"]; ok {
		newStatus = fmt.Sprint(val)
		statusChanged = oldStatus != newStatus
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updates["updated_at"] = now

	if err := s.UpdateJobFields(id, updates); err != nil {
		return Job{}, fmt.Errorf("update job: %w", err)
	}

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
