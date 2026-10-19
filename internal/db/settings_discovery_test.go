package db

import (
	"path/filepath"
	"testing"
)

// TestMigration00020_addsDiscoveryEnabledColumn pins the WP-183 migration:
// the settings table carries discovery_enabled from RunMigrations alone
// (before any settings read/write touches ensureAutopilotSettingsColumns).
func TestMigration00020_addsDiscoveryEnabledColumn(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	sqlStore, ok := s.(*SQLiteStore)
	if !ok {
		t.Fatalf("Open returned %T, want *SQLiteStore", s)
	}
	rows, err := sqlStore.Queryx(`PRAGMA table_info(settings)`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "discovery_enabled" {
			found = true
		}
	}
	if !found {
		t.Error("settings.discovery_enabled missing after RunMigrations")
	}
}

// Company discovery defaults ON (WP-183): the toggle only turns it off.
func TestSettingsDiscoveryEnabledDefaultOn(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	st, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.DiscoveryEnabled != 1 {
		t.Errorf("default DiscoveryEnabled = %d, want 1", st.DiscoveryEnabled)
	}
}

// The toggle round-trips through UpsertSettings/GetSettings.
func TestSettingsDiscoveryEnabledRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if err := s.UpsertSettings(map[string]any{"discovery_enabled": 0}); err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	st, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.DiscoveryEnabled != 0 {
		t.Errorf("DiscoveryEnabled = %d, want 0 after disable", st.DiscoveryEnabled)
	}
}
