package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
)

func TestParseToday(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"empty means no anchor", "", time.Time{}, false},
		{"valid date", "2026-08-12", time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC), false},
		{"bad format", "2026/08/12", time.Time{}, true},
		{"not a date", "hello", time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToday(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseToday(%q): expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToday(%q): unexpected error: %v", tt.in, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseToday(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestEffectiveDate(t *testing.T) {
	anchor := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	if got := effectiveDate(anchor); !got.Equal(anchor) {
		t.Errorf("effectiveDate(anchor) = %v, want %v (explicit anchor wins)", got, anchor)
	}
	zero := effectiveDate(time.Time{})
	if zero.IsZero() {
		t.Error("effectiveDate(zero) returned zero time; want machine clock fallback")
	}
}

// TestMigrateLegacyJSONShape pins the legacy scrape-cache.json shape to the
// renamed Posting type. 'waypoint scrape migrate' unmarshals this exact
// shape; a JSON-tag divergence would silently import empty rows.
func TestMigrateLegacyJSONShape(t *testing.T) {
	raw := `{
		"https://example.com/a": {
			"first_seen": "2026-08-01",
			"status": "new",
			"result": {
				"id": "1", "title": "Job A", "company": "Acme",
				"location": "Remote", "date": "2026-08-01",
				"url": "https://example.com/a",
				"description": "A great role",
				"metadata": {"level": "senior"}
			}
		},
		"https://example.com/b": {
			"first_seen": "2026-08-03",
			"status": "imported",
			"result": {"id": "2", "title": "Job B", "url": "https://example.com/b"}
		}
	}`

	var data map[string]db.Posting
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal legacy JSON: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(data))
	}

	f := db.NewFakeStore()
	imported, err := f.MigratePostings([]db.Posting{
		data["https://example.com/a"], data["https://example.com/b"],
	})
	if err != nil || imported != 2 {
		t.Fatalf("MigratePostings = %d, %v; want 2, nil", imported, err)
	}

	a, ok, _ := f.GetPosting("https://example.com/a")
	if !ok {
		t.Fatal("entry a not imported")
	}
	if a.Result.Title != "Job A" || a.Result.Company != "Acme" ||
		a.Result.Location != "Remote" || a.Result.Date != "2026-08-01" ||
		a.Result.Description != "A great role" || a.Result.Metadata["level"] != "senior" {
		t.Errorf("entry a fields lost: %+v", a.Result)
	}
	if a.FirstSeen != "2026-08-01" || a.Status != "new" {
		t.Errorf("entry a ledger fields: first_seen=%q status=%q", a.FirstSeen, a.Status)
	}

	// Legacy "imported" imports as "promoted" (pre-ledger vocabulary).
	b, _, _ := f.GetPosting("https://example.com/b")
	if b.Status != db.StatusPromoted {
		t.Errorf("legacy imported should become %q, got %q", db.StatusPromoted, b.Status)
	}
}
