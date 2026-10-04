package db

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestMigration00017_addsDisabledScrapersColumn pins the WP-175 migration:
// the settings table carries autopilot_disabled_scrapers from RunMigrations
// alone (before any settings read/write touches ensureAutopilotSettingsColumns).
func TestMigration00017_addsDisabledScrapersColumn(t *testing.T) {
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
		if name == "autopilot_disabled_scrapers" {
			found = true
		}
	}
	if !found {
		t.Error("settings.autopilot_disabled_scrapers missing after RunMigrations")
	}
}

// TestSettingsDisabledScrapersRoundTrip covers the whole seam: write through
// UpsertSettings (normalized: lowercase, trim, dedupe), read back through
// GetSettings, and emit as a JSON array (GET /api/settings shape).
func TestSettingsDisabledScrapersRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	err = s.UpsertSettings(map[string]any{
		"autopilot_disabled_scrapers": `["LinkedIn", " ncbs ", "linkedin", ""]`,
	})
	if err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}

	st, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got, want := st.AutopilotDisabledScrapers, `["linkedin","ncbs"]`; got != want {
		t.Errorf("stored = %s, want %s", got, want)
	}

	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	var view struct {
		Disabled []string `json:"autopilotDisabledScrapers"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatalf("unmarshal settings: %v\nraw: %s", err, raw)
	}
	if len(view.Disabled) != 2 || view.Disabled[0] != "linkedin" || view.Disabled[1] != "ncbs" {
		t.Errorf("JSON emission = %v, want [linkedin ncbs]", view.Disabled)
	}
}

// Fresh settings (no row) must read as an empty list, not nil/error.
func TestSettingsDisabledScrapersDefaultEmpty(t *testing.T) {
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
	if got := ParseDisabledScrapers(st.AutopilotDisabledScrapers); len(got) != 0 {
		t.Errorf("default disabled = %v, want empty", got)
	}
}

// FakeStore must behave like SQLiteStore: same normalization on write.
func TestFakeStoreDisabledScrapersRoundTrip(t *testing.T) {
	f := NewFakeStore()
	err := f.UpsertSettings(map[string]any{
		"autopilot_disabled_scrapers": `["Indeed", "indeed", " VIT "]`,
	})
	if err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	st, err := f.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got, want := st.AutopilotDisabledScrapers, `["indeed","vit"]`; got != want {
		t.Errorf("stored = %s, want %s", got, want)
	}
}

func TestParseDisabledScrapers(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", []string{}},
		{"valid list", `["a","b"]`, []string{"a", "b"}},
		{"invalid json", "{not json", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDisabledScrapers(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseDisabledScrapers(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseDisabledScrapers(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}
