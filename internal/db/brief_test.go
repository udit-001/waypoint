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
