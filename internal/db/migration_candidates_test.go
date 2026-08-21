package db

import (
	"path/filepath"
	"testing"
)

// TestMigration00013_freshDB creates the company_candidates table from
// scratch on a fresh database (all migrations run in order).
func TestMigration00013_freshDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if n, err := tableCount(s, "company_candidates"); err != nil || n != 1 {
		t.Errorf("company_candidates table missing (count=%d, err=%v)", n, err)
	}
	if n, err := indexCount(s, "idx_company_candidates_name"); err != nil || n != 1 {
		t.Errorf("unique-name index missing (count=%d, err=%v)", n, err)
	}
}
