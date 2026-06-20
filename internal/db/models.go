package db

// Job represents a job application.
type Job struct {
	ID           int64   `db:"id" json:"id"`
	Company      string  `db:"company" json:"company"`
	Position     string  `db:"position" json:"position"`
	Date         string  `db:"date" json:"date"`
	AppliedDate  string  `db:"applied_date" json:"appliedDate"`
	Status       string  `db:"status" json:"status"`
	CategoryID   int64   `db:"category_id" json:"categoryId"`
	CategoryName string  `db:"-" json:"category"` // joined from categories table, kept for API compat
	Salary       string  `db:"salary" json:"salary"`
	Location     string  `db:"location" json:"location"`
	Contact      string  `db:"contact" json:"contact"`
	URL          string  `db:"url" json:"url"`
	Notes        string  `db:"notes" json:"notes"`
	ReminderDate *string `db:"reminder_date" json:"reminderDate,omitempty"`
	CreatedAt    string  `db:"created_at" json:"createdAt"`
	UpdatedAt    string  `db:"updated_at" json:"updatedAt"`
}

// Category is a job category / tag.
type Category struct {
	ID   int64  `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

// HistoryEntry records a change to a job.
type HistoryEntry struct {
	ID        int64  `db:"id" json:"id"`
	JobID     int64  `db:"job_id" json:"jobId"`
	Action    string `db:"action" json:"action"`
	From      string `db:"from_value" json:"from"`
	To        string `db:"to_value" json:"to"`
	Timestamp string `db:"timestamp" json:"timestamp"`
}
