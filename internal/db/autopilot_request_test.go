package db

import "testing"

// TestAutopilotRunRequest_roundTrip: a requested run is consumable
// exactly once — the first Consume sees it, the second doesn't.
// This read-and-clear contract is what makes the daemon's scheduler
// poll idempotent: two polls can't double-fire one request.
func TestAutopilotRunRequest_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	got, err := s.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("Consume (absent): %v", err)
	}
	if got {
		t.Fatal("fresh store reported a run request; want none")
	}

	if err := s.RequestAutopilotRun(); err != nil {
		t.Fatalf("RequestAutopilotRun: %v", err)
	}

	got, err = s.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("Consume (present): %v", err)
	}
	if !got {
		t.Fatal("requested run not seen by first consume")
	}

	got, err = s.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("Consume (after clear): %v", err)
	}
	if got {
		t.Fatal("run request survived consumption — scheduler would re-fire")
	}
}

// TestUpsertSettingsAutopilot_freshDB: a virgin database accepts an
// autopilot enable before any settings read has run — the lazy column
// ensure must live on the write path too (regression: fresh installs
// got "no such column: autopilot_enabled" from the Settings toggle).
func TestUpsertSettingsAutopilot_freshDB(t *testing.T) {
	s := sqliteStore(t)

	if err := s.UpsertSettings(map[string]any{"autopilot_enabled": 1}); err != nil {
		t.Fatalf("UpsertSettings on fresh db: %v", err)
	}
	st, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.AutopilotEnabled != 1 {
		t.Errorf("AutopilotEnabled = %d, want 1", st.AutopilotEnabled)
	}
}
