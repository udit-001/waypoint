package db

import (
	"path/filepath"
	"testing"
)

// TestChangeEventsBus verifies the change-log bus: publish → tail by
// cursor, ordered, with the limit respected.
func TestChangeEventsBus(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if err := s.AddChangeEvent("matches"); err != nil {
		t.Fatalf("publish matches: %v", err)
	}
	if err := s.AddChangeEvent("applications"); err != nil {
		t.Fatalf("publish applications: %v", err)
	}
	if err := s.AddChangeEvent("runs"); err != nil {
		t.Fatalf("publish runs: %v", err)
	}

	evs, err := s.ChangesSince(0, 50)
	if err != nil {
		t.Fatalf("ChangesSince: %v", err)
	}
	if len(evs) != 3 {
		t.Fatalf("got %d events, want 3", len(evs))
	}
	want := []string{"matches", "applications", "runs"}
	for i, ev := range evs {
		if ev.Kind != want[i] {
			t.Errorf("event[%d].kind = %q, want %q", i, ev.Kind, want[i])
		}
		if ev.ID <= 0 {
			t.Errorf("event[%d].ID = %d, want > 0", i, ev.ID)
		}
		if ev.At == "" {
			t.Errorf("event[%d].At empty", i)
		}
	}

	// Cursor resumes after the published events.
	after, err := s.ChangesSince(evs[len(evs)-1].ID, 50)
	if err != nil {
		t.Fatalf("ChangesSince after cursor: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("got %d events after cursor, want 0", len(after))
	}

	// Limit caps the result.
	limited, err := s.ChangesSince(0, 2)
	if err != nil {
		t.Fatalf("ChangesSince limited: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("limit=2 got %d events", len(limited))
	}
}
