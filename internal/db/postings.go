package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/udit-001/waypoint/internal/dates"
	"github.com/udit-001/waypoint/internal/scraper"
)

// Posting statuses — the postings ledger's review vocabulary.
const (
	StatusNew         = "new"
	StatusShortlisted = "shortlisted"
	StatusDismissed   = "dismissed"
	StatusPromoted    = "promoted"
)

// Posting is one row of the postings ledger — a scraped job posting
// persisted by URL, the world's data, awaiting review (promote or
// dismiss). Durable dismissals are autopilot memory: the loop
// re-encounters URLs every cycle.
type Posting struct {
	FirstSeen string         `json:"first_seen"`
	Status    string         `json:"status"` // new | shortlisted | dismissed | promoted
	Result    scraper.Result `json:"result"`
}

const postingColumns = `url, title, company, location, date, description, metadata, first_seen, status`

// marshalPostingMeta serializes posting metadata for storage; "{}" when empty.
func marshalPostingMeta(meta map[string]string) (string, error) {
	if len(meta) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal posting metadata: %w", err)
	}
	return string(raw), nil
}

func scanPosting(row interface{ Scan(...any) error }) (Posting, error) {
	var (
		p           Posting
		metadataRaw string
	)
	err := row.Scan(
		&p.Result.URL, &p.Result.Title, &p.Result.Company,
		&p.Result.Location, &p.Result.Date, &p.Result.Description,
		&metadataRaw, &p.FirstSeen, &p.Status,
	)
	if err != nil {
		return Posting{}, err
	}
	if metadataRaw != "" && metadataRaw != "{}" {
		if err := json.Unmarshal([]byte(metadataRaw), &p.Result.Metadata); err != nil {
			return Posting{}, fmt.Errorf("unmarshal posting metadata: %w", err)
		}
	}
	return p, nil
}

func scanPostings(rows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}) ([]Posting, error) {
	var out []Posting
	for rows.Next() {
		p, err := scanPosting(rows)
		if err != nil {
			return nil, fmt.Errorf("scan posting: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// HasPosting returns true if a posting with the given URL is already
// in the ledger.
func (s *SQLiteStore) HasPosting(url string) (bool, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM postings WHERE url = ?", url)
	return count > 0, err
}

// AddPostings inserts new results with status "new". Results whose URL
// is already in the ledger are skipped (idempotent).
func (s *SQLiteStore) AddPostings(results []scraper.Result) error {
	now := time.Now().UTC().Format("2006-01-02")
	for _, r := range results {
		metaJSON, err := marshalPostingMeta(r.Metadata)
		if err != nil {
			return err
		}
		if _, err := s.Exec(
			`INSERT OR IGNORE INTO postings (url, title, company, location, date, description, metadata, first_seen, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'new')`,
			r.URL, r.Title, r.Company, r.Location, r.Date, r.Description, metaJSON, now,
		); err != nil {
			return fmt.Errorf("insert posting: %w", err)
		}
	}
	return nil
}

// ListPostings returns postings, optionally filtered by status.
// If status is empty, returns all entries, newest first.
func (s *SQLiteStore) ListPostings(status string) ([]Posting, error) {
	var query string
	var args []any
	if status != "" {
		query = fmt.Sprintf("SELECT %s FROM postings WHERE status = ? ORDER BY first_seen DESC", postingColumns)
		args = append(args, status)
	} else {
		query = fmt.Sprintf("SELECT %s FROM postings ORDER BY first_seen DESC", postingColumns)
	}
	rows, err := s.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPostings(rows)
}

// GetPosting returns a single posting by URL.
func (s *SQLiteStore) GetPosting(url string) (Posting, bool, error) {
	row := s.QueryRow(fmt.Sprintf("SELECT %s FROM postings WHERE url = ?", postingColumns), url)
	p, err := scanPosting(row)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return Posting{}, false, nil
		}
		return Posting{}, false, err
	}
	return p, true, nil
}

// SetPostingStatus updates the status of a posting. Idempotent.
func (s *SQLiteStore) SetPostingStatus(url, status string) error {
	result, err := s.Exec("UPDATE postings SET status = ? WHERE url = ?", status, url)
	if err != nil {
		return fmt.Errorf("set posting status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no posting with URL %q", url)
	}
	return nil
}

// PrunePostings removes entries older than days. Returns count removed.
func (s *SQLiteStore) PrunePostings(days int) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	result, err := s.Exec("DELETE FROM postings WHERE first_seen < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune postings: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// EnrichPosting updates a posting's description and merges metadata.
// Finds the entry by URL. Does not overwrite search fields (title, company,
// location, date, url). No-op if the URL isn't in the ledger.
func (s *SQLiteStore) EnrichPosting(url, desc string, meta map[string]string) error {
	p, ok, err := s.GetPosting(url)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if desc != "" {
		p.Result.Description = desc
	}
	if len(meta) > 0 {
		if p.Result.Metadata == nil {
			p.Result.Metadata = map[string]string{}
		}
		for k, v := range meta {
			p.Result.Metadata[k] = v
		}
	}
	metaJSON, err := marshalPostingMeta(p.Result.Metadata)
	if err != nil {
		return err
	}
	if _, err := s.Exec(
		"UPDATE postings SET description = ?, metadata = ? WHERE url = ?",
		p.Result.Description, metaJSON, url,
	); err != nil {
		return fmt.Errorf("enrich posting: %w", err)
	}
	return nil
}

// MigratePostings imports legacy staging entries from the JSON-file era,
// preserving their original first_seen and status values. Uses INSERT OR
// IGNORE so re-running migration is safe. Returns the number of entries
// actually inserted (skips URLs already present).
func (s *SQLiteStore) MigratePostings(entries []Posting) (int, error) {
	imported := 0
	for _, p := range entries {
		metaJSON, err := marshalPostingMeta(p.Result.Metadata)
		if err != nil {
			return imported, err
		}
		status := p.Status
		if status == "" {
			status = StatusNew
		}
		if status == "imported" { // pre-ledger vocabulary from the JSON-file era
			status = StatusPromoted
		}
		firstSeen := p.FirstSeen
		if firstSeen == "" {
			firstSeen = time.Now().UTC().Format("2006-01-02")
		}
		result, err := s.Exec(
			`INSERT OR IGNORE INTO postings (url, title, company, location, date, description, metadata, first_seen, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.Result.URL, p.Result.Title, p.Result.Company,
			p.Result.Location, p.Result.Date, p.Result.Description,
			metaJSON, firstSeen, status,
		)
		if err != nil {
			return imported, fmt.Errorf("migrate posting insert: %w", err)
		}
		if n, _ := result.RowsAffected(); n > 0 {
			imported++
		}
	}
	return imported, nil
}

// Promote moves a posting into the applications (jobs) table.
// If the URL already exists in jobs, job creation is skipped but the
// posting is still marked "promoted". The entire operation runs
// in a single transaction.
func (s *SQLiteStore) Promote(url string) (Job, error) {
	var job Job
	err := s.tx(func(tx *sqlx.Tx) error {
		// 1. Look up the posting by URL (reuses scanPosting helper).
		row := tx.QueryRowx(
			fmt.Sprintf("SELECT %s FROM postings WHERE url = ?", postingColumns),
			url,
		)
		p, err := scanPosting(row)
		if err != nil {
			if err.Error() == "sql: no rows in result set" {
				return fmt.Errorf("no posting with URL %q", url)
			}
			return fmt.Errorf("lookup posting: %w", err)
		}

		// 2. Check idempotency — skip if URL already in jobs.
		var count int
		if err := tx.Get(&count, "SELECT COUNT(*) FROM jobs WHERE url = ?", url); err != nil {
			return fmt.Errorf("check jobs for url: %w", err)
		}

		if count == 0 {
			// 3. Create the job with defaults + history (inlined IntakeAddJob
			//    because IntakeAddJob takes Store, not *sqlx.Tx).
			now := time.Now().UTC().Format(time.RFC3339)
			job = Job{
				Company:   p.Result.Company,
				Position:  p.Result.Title,
				URL:       p.Result.URL,
				Location:  p.Result.Location,
				Date:      dates.NormalizeDate(p.Result.Date),
				Status:    "Not Applied",
				CreatedAt: now,
				UpdatedAt: now,
			}
			result, err := tx.Exec(
				`INSERT INTO jobs (company, position, date, applied_date, status, category_id, salary, location, contact, url, notes, reminder_date, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
				job.Company, job.Position, job.Date, job.AppliedDate, job.Status,
				job.Salary, job.Location, job.Contact, job.URL, job.Notes,
				job.ReminderDate, job.CreatedAt, job.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("insert promoted job: %w", err)
			}
			id, _ := result.LastInsertId()
			job.ID = id

			if _, err := tx.Exec(
				`INSERT INTO history (job_id, action, from_value, to_value) VALUES (?, ?, ?, ?)`,
				job.ID, "Created", "", job.Status,
			); err != nil {
				return fmt.Errorf("add promote history: %w", err)
			}
		}

		// 4. Mark the posting as promoted.
		if _, err := tx.Exec(
			"UPDATE postings SET status = 'promoted' WHERE url = ?", url,
		); err != nil {
			return fmt.Errorf("set posting promoted: %w", err)
		}

		return nil
	})
	if err != nil {
		return Job{}, err
	}

	// Fetch the full job with category join if one was created.
	if job.ID > 0 {
		return s.GetJob(job.ID)
	}
	return job, nil
}
