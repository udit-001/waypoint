package db

import (
	"path/filepath"
	"testing"
)

func TestMigration00006_addsBriefColumns(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Fresh profile: all brief columns default empty / empty array.
	b, err := s.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief on migrated DB: %v", err)
	}
	if b.Complete {
		t.Errorf("fresh profile should be incomplete, got Complete=%v open=%v", b.Complete, b.Open)
	}

	// Write a brief via the store and read it back through the real SQLite
	// path to prove the columns round-trip.
	if err := s.UpsertProfile(map[string]any{
		"remote":              "hybrid",
		"location_preference": `["Bengaluru","Delhi"]`,
		"companies":           `["Gojek"]`,
		"keywords":            `["kotlin","android"]`,
		"current_location":    "Bengaluru",
		"seniority":           "mid",
		"visa_sponsorship":    "no",
	}); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	b, err = s.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief after write: %v", err)
	}
	if len(b.Open) != 0 {
		t.Errorf("Open = %v, want empty", b.Open)
	}
	if !b.Complete {
		t.Errorf("Complete = false, want true")
	}
	if b.Preferences.Remote != "hybrid" {
		t.Errorf("Remote = %q", b.Preferences.Remote)
	}
	if b.Facts.CurrentLocation != "Bengaluru" || b.Facts.Seniority != "mid" {
		t.Errorf("facts = %+v", b.Facts)
	}
	// List preferences are normalized (case-folded) for matching.
	if len(b.Preferences.LocationPref) != 2 || b.Preferences.LocationPref[0] != "bengaluru" {
		t.Errorf("LocationPref = %v", b.Preferences.LocationPref)
	}
}
