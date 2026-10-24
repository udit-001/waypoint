package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
)

func TestWaitForServerReadyTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/stats" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	port := portFromAddr(srv.Listener.Addr().String())
	// Server is already running — should return true quickly
	if !waitForServerReady(port, 20, 10*time.Millisecond) {
		t.Error("waitForServerReady() = false, want true (server is running)")
	}
}

func TestWaitForServerReadyFalse(t *testing.T) {
	// Port 1 is almost certainly not serving anything
	// Use small attempts/interval to keep the test fast
	if waitForServerReady(1, 3, 10*time.Millisecond) {
		t.Error("waitForServerReady(1) = true, want false (nothing on port 1)")
	}
}

func TestWaitForServerReadyNonWaypointServer(t *testing.T) {
	// A server without /api/stats should never be considered ready
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	port := portFromAddr(srv.Listener.Addr().String())
	if waitForServerReady(port, 3, 10*time.Millisecond) {
		t.Error("waitForServerReady() = true for non-waypoint server, want false")
	}
}

// TestRecoverInterruptedRuns: boot recovery closes a dead process's
// unfinished run row — the wiring behind "a crash mid-cycle must not
// leave the chip reading running nor re-run the whole sweep on restart".
// A clean ledger recovers nothing.
func TestRecoverInterruptedRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waypoint.db")
	s, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	id, err := s.AddRunLog(db.RunLog{StartedAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	if err := s.SetRunProgress(id, "sweep", "3/21", "Instem"); err != nil {
		t.Fatalf("SetRunProgress: %v", err)
	}

	recoverInterruptedRuns(s)

	last, ok, err := s.GetLastRun()
	if err != nil || !ok {
		t.Fatalf("GetLastRun: ok=%v err=%v", ok, err)
	}
	if last.Verdict != db.VerdictDegraded || last.FinishedAt == "" {
		t.Fatalf("open row not recovered: verdict=%q finishedAt=%q", last.Verdict, last.FinishedAt)
	}
}

// TestRecoverInterruptedRuns_cleanLedger: no unfinished rows — recovery
// is a no-op, so a normal boot never rewrites the runlog.
func TestRecoverInterruptedRuns_cleanLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "waypoint.db")
	s, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	done, err := s.AddRunLog(db.RunLog{
		Verdict:    db.VerdictQuiet,
		StartedAt:  "2026-01-01T09:00:00Z",
		FinishedAt: "2026-01-01T09:10:00Z",
	})
	if err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}

	recoverInterruptedRuns(s)

	last, ok, _ := s.GetLastRun()
	if !ok || last.ID != done {
		t.Fatalf("clean ledger mutated: ok=%v id=%d want %d", ok, last.ID, done)
	}
	if last.FinishedAt != "2026-01-01T09:10:00Z" {
		t.Fatalf("finished row's end time changed: %q", last.FinishedAt)
	}
}
