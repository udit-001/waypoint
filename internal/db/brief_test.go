package db

import (
	"reflect"
	"testing"
)

func TestGetBrief_emptyProfile(t *testing.T) {
	f := NewFakeStore()
	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief error: %v", err)
	}

	wantOpen := []string{"companies", "keywords", "location_preference", "remote"}
	if !reflect.DeepEqual(b.Open, wantOpen) {
		t.Errorf("empty Open = %v, want %v", b.Open, wantOpen)
	}
	if b.Complete {
		t.Errorf("empty brief should be incomplete")
	}
	if b.Preferences.LocationPref != nil || b.Preferences.Companies != nil {
		t.Errorf("empty brief preferences should hold empty lists, got %+v", b.Preferences)
	}
}

func TestGetBrief_completePreferences(t *testing.T) {
	f := NewFakeStore()
	// Fill only the preferences bucket. Facts and constraints stay empty.
	f.Profile = Profile{
		Remote:       "hybrid",
		LocationPref: `["Bengaluru","Delhi"]`,
		Companies:    `["Gojek","Flipkart"]`,
		Keywords:     `["android","kotlin"]`,
		Dealbreakers: `["night shift"]`,
	}

	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief error: %v", err)
	}

	if len(b.Open) != 0 {
		t.Errorf("Open = %v, want empty (preferences full)", b.Open)
	}
	if !b.Complete {
		t.Errorf("preferences full should be complete")
	}

	// Facts must not gate completion.
	if b.Facts.Seniority != "" || b.Constraints.VisaSponsorship != "" {
		t.Errorf("facts/constraints should still be empty, got %+v", b)
	}

	if !reflect.DeepEqual(b.Preferences.LocationPref, []string{"Bengaluru", "Delhi"}) {
		t.Errorf("LocationPref = %v", b.Preferences.LocationPref)
	}
	if !reflect.DeepEqual(b.Preferences.Companies, []string{"Gojek", "Flipkart"}) {
		t.Errorf("Companies = %v", b.Preferences.Companies)
	}
}

func TestGetBrief_partialFactsAndConstraints(t *testing.T) {
	f := NewFakeStore()
	// Full preferences + some facts/constraints seeded.
	f.Profile = Profile{
		Title:           "Bioinformatics Researcher",
		Seniority:       "mid",
		CurrentLocation: "Bengaluru",
		VisaSponsorship: "no",
		SalaryFloor:     `{"region":"IN","amount":0}`,
		Remote:          "onsite",
		LocationPref:    `["India"]`,
		Companies:       `["NCBS"]`,
		Keywords:        `["genomics"]`,
	}

	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief error: %v", err)
	}

	if len(b.Open) != 0 {
		t.Errorf("Open = %v, want empty", b.Open)
	}
	if !b.Complete {
		t.Errorf("should be complete")
	}
	if b.Facts.Title != "Bioinformatics Researcher" || b.Facts.Seniority != "mid" {
		t.Errorf("facts = %+v", b.Facts)
	}
	if b.Facts.Skills != nil {
		t.Errorf("empty skills should be nil, got %v", b.Facts.Skills)
	}
}

func TestGetBrief_seniorityPreferStoredOverDerived(t *testing.T) {
	f := NewFakeStore()
	f.Profile = Profile{
		Seniority:  "senior", // stored value wins
		Experience: `["1 year at NCBS"]`,
		Remote:     "onsite",
		Companies:  `["NCBS"]`,
		Keywords:   `["genomics"]`,
		// location_preference left empty → one open item.
	}

	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}
	if b.Facts.Seniority != "senior" {
		t.Errorf("stored seniority should win, got %q", b.Facts.Seniority)
	}
}

func TestGetBrief_seniorityDerivedFromExperience(t *testing.T) {
	f := NewFakeStore()
	f.Profile = Profile{
		Experience: `["8 years in genomics"]`,
		Remote:     "onsite",
		Companies:  `["NCBS"]`,
		Keywords:   `["genomics"]`,
	}

	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}
	if b.Facts.Seniority != "senior" {
		t.Errorf("expected derived seniority 'senior', got %q", b.Facts.Seniority)
	}
}

func TestGetBrief_salaryFloorCurrencyDerived(t *testing.T) {
	f := NewFakeStore()
	f.Profile = Profile{
		SalaryFloor: `[{"region":"IN","amount":100000},{"region":"GB","amount":30000}]`,
		Remote:      "onsite",
		Companies:   `["NCBS"]`,
		Keywords:    `["genomics"]`,
	}

	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}
	entries := b.Constraints.SalaryFloor
	if len(entries) != 2 {
		t.Fatalf("expected 2 salary entries, got %d", len(entries))
	}
	if entries[0].Currency != "INR" || entries[1].Currency != "GBP" {
		t.Errorf("currencies = %q, %q; want INR, GBP", entries[0].Currency, entries[1].Currency)
	}
}

func TestGetBrief_optionalRefinementsDoNotGate(t *testing.T) {
	f := NewFakeStore()
	// dealbreakers and avoid_companies are optional refinements — empty
	// (meaning "none") is a valid settled answer and does not gate completion.
	f.Profile = Profile{
		Remote:       "remote",
		LocationPref: `["Anywhere"]`,
		Companies:    `["Gojek"]`,
		Keywords:     `["kotlin"]`,
		// Dealbreakers and AvoidCompanies intentionally empty.
	}

	b, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief error: %v", err)
	}
	if len(b.Open) != 0 {
		t.Errorf("empty refinements should not be open, got %v", b.Open)
	}
	if !b.Complete {
		t.Errorf("should be complete despite empty dealbreakers/avoid_companies")
	}
}
