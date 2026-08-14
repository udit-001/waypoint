package db

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestMigration00007_dropsEmailStyleAndUpgradesExperience(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// greeting_style / sign_off columns are gone.
	var n int
	if err := s.(*SQLiteStore).Get(&n, `SELECT count(*) FROM pragma_table_info('profile') WHERE name IN ('greeting_style','sign_off')`); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if n != 0 {
		t.Errorf("greeting_style/sign_off columns still present (count=%d)", n)
	}

	// Legacy flat-string experience upgrades to structured entries on read.
	if err := s.UpsertProfile(map[string]any{
		"experience": `["5 years backend development","Intern"]`,
	}); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}
	p, err := s.GetProfile()
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	want := []ExperienceEntry{{Title: "5 years backend development"}, {Title: "Intern"}}
	if got := ParseExperienceEntries(p.Experience); !reflect.DeepEqual(got, want) {
		t.Errorf("upgraded experience = %#v, want %#v", got, want)
	}
	// And the derived seniority still reads from it.
	if got := DeriveSeniority(p.Experience); got != "mid" {
		t.Errorf("DeriveSeniority = %q, want mid", got)
	}
}
