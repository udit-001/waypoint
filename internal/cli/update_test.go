package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/udit-001/waypoint/internal/db"
)

func resetUpdateFlags() {
	updateFlags.company = ""
	updateFlags.position = ""
	updateFlags.status = ""
	updateFlags.category = ""
	updateFlags.salary = ""
	updateFlags.location = ""
	updateFlags.contact = ""
	updateFlags.url = ""
	updateFlags.notes = ""
	updateFlags.notesFile = ""
	updateFlags.date = ""
	updateFlags.appliedDate = ""
	updateFlags.reminderDate = ""
	updateCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
}

func TestUpdateClearFields(t *testing.T) {
	tests := []struct {
		name   string
		flags  map[string]string
		verify func(t *testing.T, j db.Job)
	}{
		{
			name:  "clear date with empty string",
			flags: map[string]string{"date": ""},
			verify: func(t *testing.T, j db.Job) {
				if j.Date != "" {
					t.Errorf("Date = %q, want empty", j.Date)
				}
			},
		},
		{
			name:  "clear notes with empty string",
			flags: map[string]string{"notes": ""},
			verify: func(t *testing.T, j db.Job) {
				if j.Notes != "" {
					t.Errorf("Notes = %q, want empty", j.Notes)
				}
			},
		},
		{
			name:  "clear category with empty string sets uncategorized",
			flags: map[string]string{"category": ""},
			verify: func(t *testing.T, j db.Job) {
				if j.CategoryID != 0 {
					t.Errorf("CategoryID = %d, want 0", j.CategoryID)
				}
			},
		},
		{
			name:  "clear applied_date with empty string",
			flags: map[string]string{"applied-date": ""},
			verify: func(t *testing.T, j db.Job) {
				if j.AppliedDate != "" {
					t.Errorf("AppliedDate = %q, want empty", j.AppliedDate)
				}
			},
		},
		{
			name:  "non-empty update still works",
			flags: map[string]string{"company": "Google"},
			verify: func(t *testing.T, j db.Job) {
				if j.Company != "Google" {
					t.Errorf("Company = %q, want %q", j.Company, "Google")
				}
			},
		},
		{
			name:  "omitting flag leaves field untouched",
			flags: map[string]string{"company": "NewCo"},
			verify: func(t *testing.T, j db.Job) {
				if j.Date != "2026-07-15" {
					t.Errorf("Date = %q, want %q (unchanged)", j.Date, "2026-07-15")
				}
				if j.Notes != "existing notes" {
					t.Errorf("Notes = %q, want %q (unchanged)", j.Notes, "existing notes")
				}
			},
		},
		{
			name:  "clear multiple fields at once",
			flags: map[string]string{"date": "", "notes": ""},
			verify: func(t *testing.T, j db.Job) {
				if j.Date != "" {
					t.Errorf("Date = %q, want empty", j.Date)
				}
				if j.Notes != "" {
					t.Errorf("Notes = %q, want empty", j.Notes)
				}
			},
		},
		{
			name:  "clear date and update company simultaneously",
			flags: map[string]string{"date": "", "company": "NewCorp"},
			verify: func(t *testing.T, j db.Job) {
				if j.Date != "" {
					t.Errorf("Date = %q, want empty", j.Date)
				}
				if j.Company != "NewCorp" {
					t.Errorf("Company = %q, want %q", j.Company, "NewCorp")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetUpdateFlags()
			fake := db.NewFakeStore()
			fake.Jobs[1] = db.Job{
				ID:          1,
				Company:     "TestCo",
				Position:    "Engineer",
				Date:        "2026-07-15",
				AppliedDate: "2026-06-01",
				Status:      "Applied",
				CategoryID:  2,
				Notes:       "existing notes",
				URL:         "https://example.com",
			}
			fake.Categories[2] = db.Category{ID: 2, Name: "Engineering"}
			store = fake
			jsonOut = false

			for name, val := range tc.flags {
				if err := updateCmd.Flags().Set(name, val); err != nil {
					t.Fatalf("failed to set flag %s: %v", name, err)
				}
			}

			if err := updateCmd.RunE(updateCmd, []string{"1"}); err != nil {
				t.Fatalf("RunE error: %v", err)
			}

			updated, err := fake.GetJob(1)
			if err != nil {
				t.Fatalf("GetJob error: %v", err)
			}
			tc.verify(t, updated)
		})
	}
}

func TestUpdateNoFlagsChanged(t *testing.T) {
	resetUpdateFlags()
	fake := db.NewFakeStore()
	fake.Jobs[1] = db.Job{ID: 1, Company: "TestCo", Position: "Engineer"}
	store = fake
	jsonOut = false

	err := updateCmd.RunE(updateCmd, []string{"1"})
	if err == nil {
		t.Error("expected error when no flags changed, got nil")
	}
}
