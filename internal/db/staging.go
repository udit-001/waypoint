package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

const stagingColumns = `url, title, company, location, date, description, metadata, first_seen, status`

func scanStagedResult(row interface{ Scan(...any) error }) (scraper.StagedResult, error) {
	var (
		sr          scraper.StagedResult
		metadataRaw string
	)
	err := row.Scan(
		&sr.Result.URL, &sr.Result.Title, &sr.Result.Company,
		&sr.Result.Location, &sr.Result.Date, &sr.Result.Description,
		&metadataRaw, &sr.FirstSeen, &sr.Status,
	)
	if err != nil {
		return scraper.StagedResult{}, err
	}
	if metadataRaw != "" && metadataRaw != "{}" {
		if err := json.Unmarshal([]byte(metadataRaw), &sr.Result.Metadata); err != nil {
			return scraper.StagedResult{}, fmt.Errorf("unmarshal staging metadata: %w", err)
		}
	}
	return sr, nil
}

func scanStagedResults(rows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}) ([]scraper.StagedResult, error) {
	var out []scraper.StagedResult
	for rows.Next() {
		sr, err := scanStagedResult(rows)
		if err != nil {
			return nil, fmt.Errorf("scan staged result: %w", err)
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}

// IsSeen returns true if a result with the given URL is already staged.
func (s *SQLiteStore) IsSeen(url string) (bool, error) {
	var count int
	err := s.Get(&count, "SELECT COUNT(*) FROM scrape_staging WHERE url = ?", url)
	return count > 0, err
}

// AddStaging inserts new results with status "new". Results whose URL is
// already staged are skipped (idempotent).
func (s *SQLiteStore) AddStaging(results []scraper.Result) error {
	now := time.Now().UTC().Format("2006-01-02")
	for _, r := range results {
		metaJSON := "{}"
		if len(r.Metadata) > 0 {
			raw, err := json.Marshal(r.Metadata)
			if err != nil {
				return fmt.Errorf("marshal staging metadata: %w", err)
			}
			metaJSON = string(raw)
		}
		if _, err := s.Exec(
			`INSERT OR IGNORE INTO scrape_staging (url, title, company, location, date, description, metadata, first_seen, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'new')`,
			r.URL, r.Title, r.Company, r.Location, r.Date, r.Description, metaJSON, now,
		); err != nil {
			return fmt.Errorf("insert staging: %w", err)
		}
	}
	return nil
}

// ListStaging returns staged results, optionally filtered by status.
// If status is empty, returns all entries, newest first.
func (s *SQLiteStore) ListStaging(status string) ([]scraper.StagedResult, error) {
	var query string
	var args []any
	if status != "" {
		query = fmt.Sprintf("SELECT %s FROM scrape_staging WHERE status = ? ORDER BY first_seen DESC", stagingColumns)
		args = append(args, status)
	} else {
		query = fmt.Sprintf("SELECT %s FROM scrape_staging ORDER BY first_seen DESC", stagingColumns)
	}
	rows, err := s.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStagedResults(rows)
}

// GetStaged returns a single staged result by URL.
func (s *SQLiteStore) GetStaged(url string) (scraper.StagedResult, bool, error) {
	row := s.QueryRow(fmt.Sprintf("SELECT %s FROM scrape_staging WHERE url = ?", stagingColumns), url)
	sr, err := scanStagedResult(row)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return scraper.StagedResult{}, false, nil
		}
		return scraper.StagedResult{}, false, err
	}
	return sr, true, nil
}

// SetStagingStatus updates the status of a staged result. Idempotent.
func (s *SQLiteStore) SetStagingStatus(url, status string) error {
	result, err := s.Exec("UPDATE scrape_staging SET status = ? WHERE url = ?", status, url)
	if err != nil {
		return fmt.Errorf("set staging status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no staged result with URL %q", url)
	}
	return nil
}

// PruneStaging removes entries older than days. Returns count removed.
func (s *SQLiteStore) PruneStaging(days int) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	result, err := s.Exec("DELETE FROM scrape_staging WHERE first_seen < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune staging: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// EnrichStaging updates a staged result's description and merges metadata.
// Finds the entry by URL. Does not overwrite search fields (title, company,
// location, date, url). No-op if the URL isn't staged.
func (s *SQLiteStore) EnrichStaging(url, desc string, meta map[string]string) error {
	sr, ok, err := s.GetStaged(url)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if desc != "" {
		sr.Result.Description = desc
	}
	if len(meta) > 0 {
		if sr.Result.Metadata == nil {
			sr.Result.Metadata = map[string]string{}
		}
		for k, v := range meta {
			sr.Result.Metadata[k] = v
		}
	}
	metaJSON := "{}"
	if len(sr.Result.Metadata) > 0 {
		raw, err := json.Marshal(sr.Result.Metadata)
		if err != nil {
			return fmt.Errorf("marshal staging metadata: %w", err)
		}
		metaJSON = string(raw)
	}
	if _, err := s.Exec(
		"UPDATE scrape_staging SET description = ?, metadata = ? WHERE url = ?",
		sr.Result.Description, metaJSON, url,
	); err != nil {
		return fmt.Errorf("enrich staging: %w", err)
	}
	return nil
}
