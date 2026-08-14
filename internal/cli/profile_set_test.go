package cli

import (
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

func TestProfileSetBriefFields(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	flags := []string{
		"remote", "hybrid",
		"location-preference", "Bengaluru,Delhi",
		"companies", "Gojek,Flipkart",
		"keywords", "[\"kotlin\",\"android\"]",
		"current-location", "Bengaluru",
		"visa-sponsorship", "no",
		"salary-floor", "IN:100000,GB:30000",
	}
	for i := 0; i < len(flags); i += 2 {
		if err := profileSetCmd.Flags().Set(flags[i], flags[i+1]); err != nil {
			t.Fatalf("Set flag %s: %v", flags[i], err)
		}
	}

	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}

	p, _ := fake.GetProfile()
	if p.Remote != "hybrid" {
		t.Errorf("Remote = %q", p.Remote)
	}
	if p.CurrentLocation != "Bengaluru" {
		t.Errorf("CurrentLocation = %q", p.CurrentLocation)
	}
	if p.VisaSponsorship != "no" {
		t.Errorf("VisaSponsorship = %q", p.VisaSponsorship)
	}
	// Stored lists are normalized (case-folded).
	if p.Companies != `["gojek","flipkart"]` {
		t.Errorf("Companies = %q", p.Companies)
	}
	if p.Keywords != `["kotlin","android"]` {
		t.Errorf("Keywords = %q", p.Keywords)
	}
	if p.LocationPref != `["bengaluru","delhi"]` {
		t.Errorf("LocationPref = %q", p.LocationPref)
	}
	// Salary is stored as {region, amount} with NO currency field.
	if p.SalaryFloor != `[{"region":"IN","amount":100000},{"region":"GB","amount":30000}]` {
		t.Errorf("SalaryFloor = %q", p.SalaryFloor)
	}
}

func TestProfileSetSeniorityGate(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	// No experience → manual seniority is allowed (placeholder before resume).
	if err := profileSetCmd.Flags().Set("seniority", "mid"); err != nil {
		t.Fatalf("Set seniority: %v", err)
	}
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE (no experience): %v", err)
	}
	if p, _ := fake.GetProfile(); p.Seniority != "mid" {
		t.Errorf("expected seniority stored when no experience, got %q", p.Seniority)
	}

	// Experience with a year signal → manual seniority is rejected.
	fake.UpsertProfile(map[string]any{"experience": `["8 years in genomics"]`})
	if err := profileSetCmd.Flags().Set("seniority", "junior"); err != nil {
		t.Fatalf("Set seniority: %v", err)
	}
	err := profileSetCmd.RunE(profileSetCmd, nil)
	if err == nil {
		t.Fatal("expected error when experience derives seniority, got nil")
	}
	if p, _ := fake.GetProfile(); p.Seniority != "mid" {
		t.Errorf("stored seniority should be unchanged, got %q", p.Seniority)
	}
}

func TestProfileSetClearViaChanged(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	// Set a value first.
	if err := profileSetCmd.Flags().Set("remote", "hybrid"); err != nil {
		t.Fatalf("Set remote: %v", err)
	}
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE (set): %v", err)
	}
	if p, _ := fake.GetProfile(); p.Remote != "hybrid" {
		t.Fatalf("expected remote=hybrid after set, got %q", p.Remote)
	}

	// Explicitly clear it with an empty value.
	if err := profileSetCmd.Flags().Set("remote", ""); err != nil {
		t.Fatalf("Set remote empty: %v", err)
	}
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE (clear): %v", err)
	}
	p, _ := fake.GetProfile()
	if p.Remote != "" {
		t.Errorf("expected remote cleared to '', got %q", p.Remote)
	}

	// Clearing should flip it back to open in the brief.
	b, _ := fake.GetBrief()
	for _, o := range b.Open {
		if o == "remote" {
			return
		}
	}
	t.Errorf("expected 'remote' back in brief open after clear, got %v", b.Open)
}

func TestProfileSetSalaryFloorAmountOnlyDefaultsToCurrentLocation(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	if err := profileSetCmd.Flags().Set("current-location", "IN"); err != nil {
		t.Fatalf("Set current-location: %v", err)
	}
	if err := profileSetCmd.Flags().Set("salary-floor", "100000"); err != nil {
		t.Fatalf("Set salary-floor: %v", err)
	}
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	p, _ := fake.GetProfile()
	if p.SalaryFloor != `[{"region":"IN","amount":100000}]` {
		t.Errorf("SalaryFloor = %q, want region defaulted to current-location", p.SalaryFloor)
	}
}

func TestProfileSetBriefJSONOutput(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = true

	if err := profileSetCmd.Flags().Set("remote", "remote"); err != nil {
		t.Fatalf("Set remote: %v", err)
	}
	out := captureStdout(t, func() {
		if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
	})
	if out == "" {
		t.Fatal("expected JSON output, got empty")
	}
	if !strings.Contains(out, "remote") {
		t.Errorf("JSON output missing remote field: %q", out)
	}
}
