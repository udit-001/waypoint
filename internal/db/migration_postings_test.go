package db

import (
	"path/filepath"
	"testing"
)

func tableCount(s Store, name string) (int, error) {
	var n int
	err := s.(*SQLiteStore).Get(&n,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name)
	return n, err
}

func indexCount(s Store, name string) (int, error) {
	var n int
	err := s.(*SQLiteStore).Get(&n,
		`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, name)
	return n, err
}

// TestMigration00008_freshDB creates the postings ledger from scratch:
// V4 still creates scrape_staging, V8 renames it in place.
func TestMigration00008_freshDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if n, err := tableCount(s, "postings"); err != nil || n != 1 {
		t.Errorf("postings table missing (count=%d, err=%v)", n, err)
	}
	if n, err := tableCount(s, "scrape_staging"); err != nil || n != 0 {
		t.Errorf("scrape_staging should be gone after migration (count=%d, err=%v)", n, err)
	}
	for _, idx := range []string{"idx_postings_status", "idx_postings_first_seen"} {
		if n, err := indexCount(s, idx); err != nil || n != 1 {
			t.Errorf("index %s missing (count=%d, err=%v)", idx, n, err)
		}
	}
	for _, idx := range []string{"idx_scrape_staging_status", "idx_scrape_staging_first_seen"} {
		if n, err := indexCount(s, idx); err != nil || n != 0 {
			t.Errorf("stale index %s still present (count=%d, err=%v)", idx, n, err)
		}
	}
}

// TestMigration00008_existingDB migrates a pre-V8 database in place:
// the scrape_staging table (with old statuses and data) becomes the
// postings ledger, converting "imported" rows to "promoted".
func TestMigration00008_existingDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Migrate fully, then revert to the pre-V8 shape: table scrape_staging,
	// old index names, goose at version 7. This reproduces exactly what a
	// database last touched by V7 looks like — everything V8 cares about.
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	raw := s.(*SQLiteStore)
	for _, stmt := range []string{
		`ALTER TABLE postings RENAME TO scrape_staging`,
		`DROP INDEX IF EXISTS idx_postings_status`,
		`DROP INDEX IF EXISTS idx_postings_first_seen`,
		`CREATE INDEX IF NOT EXISTS idx_scrape_staging_status ON scrape_staging(status)`,
		`CREATE INDEX IF NOT EXISTS idx_scrape_staging_first_seen ON scrape_staging(first_seen)`,
		// Roll back everything after V8 so the shape matches a real
		// V7-era database as later migrations land.
		`DROP TABLE IF EXISTS company_candidates`,
		`DELETE FROM goose_db_version WHERE version_id = 8`,
		`DELETE FROM goose_db_version WHERE version_id = 9`,
		`DELETE FROM goose_db_version WHERE version_id = 10`,
		`DELETE FROM goose_db_version WHERE version_id = 11`,
		`DELETE FROM goose_db_version WHERE version_id = 12`,
		`DELETE FROM goose_db_version WHERE version_id = 13`,
		`DELETE FROM goose_db_version WHERE version_id = 14`,
		// V16 added jobs.review_json; ADD COLUMN can't be un-applied by
		// deleting the version row — drop the column itself.
		`ALTER TABLE jobs DROP COLUMN review_json`,
		`DELETE FROM goose_db_version WHERE version_id = 15`,
		`DELETE FROM goose_db_version WHERE version_id = 16`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("revert to V7 (%q): %v", stmt, err)
		}
	}

	// Seed rows in every legacy status.
	seed := []string{
		`INSERT INTO scrape_staging (url, title, company, location, date, description, metadata, first_seen, status)
		 VALUES ('https://example.com/new', 'Job A', 'Corp', 'Remote', '2026-08-01', 'desc', '{"level":"senior"}', '2026-08-01', 'new')`,
		`INSERT INTO scrape_staging (url, title, company, first_seen, status)
		 VALUES ('https://example.com/dismissed', 'Job B', 'Corp', '2026-08-02', 'dismissed')`,
		`INSERT INTO scrape_staging (url, title, company, first_seen, status)
		 VALUES ('https://example.com/imported', 'Job C', 'Corp', '2026-08-03', 'imported')`,
	}
	for _, stmt := range seed {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("seed (%q): %v", stmt, err)
		}
	}

	// Re-run migrations — V8 applies to the old-shape DB.
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations (upgrade in place): %v", err)
	}

	if n, err := tableCount(s, "scrape_staging"); err != nil || n != 0 {
		t.Errorf("scrape_staging should be renamed (count=%d, err=%v)", n, err)
	}

	// Statuses: new/dismissed preserved, imported converted to promoted.
	want := map[string]string{
		"https://example.com/new":       "new",
		"https://example.com/dismissed": "dismissed",
		"https://example.com/imported":  "promoted",
	}
	for url, status := range want {
		var got string
		if err := raw.Get(&got, `SELECT status FROM postings WHERE url = ?`, url); err != nil {
			t.Fatalf("read %s: %v", url, err)
		}
		if got != status {
			t.Errorf("%s: status = %q, want %q", url, got, status)
		}
	}

	// Data survives the rename.
	var title, meta string
	if err := raw.Get(&title, `SELECT title FROM postings WHERE url = 'https://example.com/new'`); err != nil || title != "Job A" {
		t.Errorf("title preserved = %q (err=%v)", title, err)
	}
	if err := raw.Get(&meta, `SELECT metadata FROM postings WHERE url = 'https://example.com/new'`); err != nil || meta != `{"level":"senior"}` {
		t.Errorf("metadata preserved = %q (err=%v)", meta, err)
	}
}
