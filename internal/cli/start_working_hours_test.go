package cli

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// windowAroundNow returns an ordered two-hour work window that excludes
// the current local minute, so the out-of-window case never depends on
// the wall clock.
func windowAroundNow() (int, int) {
	m := time.Now().Hour()*60 + time.Now().Minute()
	if m < 1260 {
		return m + 120, m + 170
	}
	return m - 200, m - 150
}

// TestAutopilotDueWorkingHours: the daemon's poll wrapper builds the
// Schedule from settings and the ledger's "new" depth (WP-196) — the
// policy CycleDue pins in unit tests, here across the store seam.
func TestAutopilotDueWorkingHours(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waypoint.db")
	s, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Settings helper: enabled, in-window covering now, default rates.
	setInWindow := func(t *testing.T) {
		t.Helper()
		m := time.Now().Hour()*60 + time.Now().Minute()
		start, end := m-30, m+30
		if start < 0 {
			start = 0
		}
		if end > 1439 {
			end = 1439
		}
		err := s.UpsertSettings(map[string]any{
			"autopilot_enabled":         1,
			"autopilot_work_start":      start,
			"autopilot_work_end":        end,
			"autopilot_working_cadence": 45,
		})
		if err != nil {
			t.Fatalf("UpsertSettings: %v", err)
		}
	}
	finishRunAgo := func(t *testing.T, minutes int) {
		t.Helper()
		fin := time.Now().Add(-time.Duration(minutes) * time.Minute).UTC().Format(time.RFC3339)
		if _, err := s.AddRunLog(db.RunLog{Verdict: db.VerdictQuiet, StartedAt: fin, FinishedAt: fin}); err != nil {
			t.Fatalf("AddRunLog: %v", err)
		}
	}

	t.Run("drained fires on the working cadence", func(t *testing.T) {
		setInWindow(t)
		finishRunAgo(t, 45)
		due, reason := autopilotDue(s)
		if !due || reason != "cadence due" {
			t.Errorf("autopilotDue = %v (%q), want true (cadence due)", due, reason)
		}
	})

	t.Run("drained waits under the working cadence", func(t *testing.T) {
		setInWindow(t)
		finishRunAgo(t, 10)
		if due, _ := autopilotDue(s); due {
			t.Error("autopilotDue = true after 10m, want false (45m working cadence)")
		}
	})

	t.Run("backlog drains on the drain interval", func(t *testing.T) {
		setInWindow(t)
		finishRunAgo(t, 35)
		if err := s.AddPostings([]scraper.Result{
			{URL: "https://example.com/a", Title: "A", Company: "Corp"},
			{URL: "https://example.com/b", Title: "B", Company: "Corp"},
		}); err != nil {
			t.Fatalf("AddPostings: %v", err)
		}
		if n, err := s.CountPostings(db.StatusNew); err != nil || n < 2 {
			t.Fatalf("CountPostings(new) = %d, %v; want >= 2", n, err)
		}
		// 35m is under the 45m working cadence: only the drain interval
		// (30m, backlog > 0) can make this due.
		if due, reason := autopilotDue(s); !due || reason != "cadence due" {
			t.Errorf("autopilotDue = %v (%q), want true via drain interval", due, reason)
		}
	})

	t.Run("outside the window never fires", func(t *testing.T) {
		start, end := windowAroundNow()
		err := s.UpsertSettings(map[string]any{
			"autopilot_enabled":    1,
			"autopilot_work_start": start,
			"autopilot_work_end":   end,
		})
		if err != nil {
			t.Fatalf("UpsertSettings: %v", err)
		}
		finishRunAgo(t, 8*60)
		due, reason := autopilotDue(s)
		if due || reason != "outside work window" {
			t.Errorf("autopilotDue = %v (%q), want false (outside work window)", due, reason)
		}
	})

	t.Run("a failed ledger read polls as drained", func(t *testing.T) {
		setInWindow(t)
		finishRunAgo(t, 35)
		if err := s.AddPostings([]scraper.Result{
			{URL: "https://example.com/c", Title: "C", Company: "Corp"},
		}); err != nil {
			t.Fatalf("AddPostings: %v", err)
		}
		// With the real backlog this would fire via the 30m drain
		// interval; the failing count must poll as Backlog 0 instead —
		// 35m sits under the 45m working cadence, so not due.
		if due, _ := autopilotDue(errCountStore{Store: s}); due {
			t.Error("autopilotDue = true with a failing ledger read, want false (backlog 0)")
		}
	})
}

// errCountStore forces the ledger-depth read to fail: autopilotDue must
// poll as Backlog 0 and log, never block the scheduler.
type errCountStore struct {
	db.Store
}

var errCountBoom = errors.New("count boom")

func (e errCountStore) CountPostings(string) (int, error) { return 0, errCountBoom }
