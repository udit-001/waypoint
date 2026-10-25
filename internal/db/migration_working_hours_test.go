package db

import (
	"path/filepath"
	"testing"
)

// TestMigration00022_addsWorkingHoursColumns pins the WP-196 migration:
// the settings table carries the work window and working cadence from
// RunMigrations alone (before any settings read touches
// ensureAutopilotSettingsColumns).
func TestMigration00022_addsWorkingHoursColumns(t *testing.T) {
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
	found := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		found[name] = true
	}
	for _, col := range []string{"autopilot_work_start", "autopilot_work_end", "autopilot_working_cadence"} {
		if !found[col] {
			t.Errorf("settings.%s missing after RunMigrations", col)
		}
	}
}

// Working-hours defaults (WP-196): a fresh settings row reads 09:00–18:00
// and a 45-minute working cadence; an update round-trips.
func TestSettingsWorkingHoursRoundTrip(t *testing.T) {
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
	if st.AutopilotWorkStart != 540 || st.AutopilotWorkEnd != 1080 {
		t.Errorf("default window = %d–%d, want 540–1080", st.AutopilotWorkStart, st.AutopilotWorkEnd)
	}
	if st.AutopilotWorkingCadence != 45 {
		t.Errorf("default working cadence = %d, want 45", st.AutopilotWorkingCadence)
	}

	updates := map[string]any{
		"autopilot_work_start":      600,
		"autopilot_work_end":        1200,
		"autopilot_working_cadence": 30,
	}
	if err := s.UpsertSettings(updates); err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	st, err = s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}
	if st.AutopilotWorkStart != 600 || st.AutopilotWorkEnd != 1200 {
		t.Errorf("window after update = %d–%d, want 600–1200", st.AutopilotWorkStart, st.AutopilotWorkEnd)
	}
	if st.AutopilotWorkingCadence != 30 {
		t.Errorf("working cadence after update = %d, want 30", st.AutopilotWorkingCadence)
	}
}

// Same round trip through the fake store, which mirrors the SQLite
// columnMap by hand.
func TestFakeStoreWorkingHoursRoundTrip(t *testing.T) {
	f := NewFakeStore()
	if f.Settings.AutopilotWorkStart != 540 || f.Settings.AutopilotWorkEnd != 1080 {
		t.Errorf("fake default window = %d–%d, want 540–1080", f.Settings.AutopilotWorkStart, f.Settings.AutopilotWorkEnd)
	}
	err := f.UpsertSettings(map[string]any{
		"autopilot_work_start":      480,
		"autopilot_work_end":        1020,
		"autopilot_working_cadence": 60,
	})
	if err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	st, _ := f.GetSettings()
	if st.AutopilotWorkStart != 480 || st.AutopilotWorkEnd != 1020 || st.AutopilotWorkingCadence != 60 {
		t.Errorf("fake round trip = %d–%d @%d, want 480–1020 @60",
			st.AutopilotWorkStart, st.AutopilotWorkEnd, st.AutopilotWorkingCadence)
	}
}
