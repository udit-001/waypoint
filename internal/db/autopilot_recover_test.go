package db

import (
	"strings"
	"testing"
	"time"
)

// runRecoveryCases is one scenario run against every Store adapter: an
// unfinished row whose newest sibling is a normally finished run. The
// store seam has two implementations (SQLite, FakeStore); this test is
// the small conformance slice that keeps them from drifting — if an
// adapter doesn't recover the same facts, the daemon's boot recovery and
// CycleDue's "an unfinished row is the cycle running now" invariant
// disagree about reality.
func runRecoveryCases(t *testing.T) map[string]Store {
	t.Helper()
	stores := map[string]Store{
		"sqlite": sqliteStore(t),
		"fake":   &FakeStore{},
	}
	return stores
}

// TestCloseInterruptedRuns_recoverAndClearStage: a dead cycle's row
// closes as degraded with a stage error, an end time, its duration
// measured, and — never skipped — its live stage facts cleared, so a
// closed row is the run record, not a progress display. A finished row
// beside it passes through untouched.
func TestCloseInterruptedRuns_recoverAndClearStage(t *testing.T) {
	for name, store := range runRecoveryCases(t) {
		t.Run(name, func(t *testing.T) {
			done, err := store.AddRunLog(RunLog{
				Verdict:     VerdictQuiet,
				StartedAt:   "2026-01-01T09:00:00Z",
				FinishedAt:  "2026-01-01T09:10:00Z",
				DurationMs:  600_000,
				PostingsNew: 3,
			})
			if err != nil {
				t.Fatalf("AddRunLog (finished): %v", err)
			}

			open, err := store.AddRunLog(RunLog{
				StartedAt: "2026-01-01T10:00:00Z",
			})
			if err != nil {
				t.Fatalf("AddRunLog (open): %v", err)
			}
			if err := store.SetRunProgress(open, "sweep", "3/21", "Instem"); err != nil {
				t.Fatalf("SetRunProgress: %v", err)
			}

			now := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
			closed, err := store.CloseInterruptedRuns(now)
			if err != nil {
				t.Fatalf("CloseInterruptedRuns: %v", err)
			}
			if closed != 1 {
				t.Fatalf("closed %d rows, want 1", closed)
			}

			got, ok, err := store.GetLastRun()
			if err != nil || !ok {
				t.Fatalf("GetLastRun: ok=%v err=%v", ok, err)
			}
			if got.ID != open {
				t.Fatalf("recovered wrong row: id=%d want %d", got.ID, open)
			}
			if got.Verdict != VerdictDegraded {
				t.Fatalf("verdict %q, want %q — partial coverage must not read as dry", got.Verdict, VerdictDegraded)
			}
			if got.FinishedAt == "" {
				t.Fatal("recovered row still unfinished")
			}
			started, _ := time.Parse(time.RFC3339, got.StartedAt)
			if got.DurationMs != now.Sub(started).Milliseconds() {
				t.Fatalf("duration %dms, want %d", got.DurationMs, now.Sub(started).Milliseconds())
			}
			if got.Stage != "" || got.StageOf != "" || got.StageLabel != "" {
				t.Fatalf("live stage facts survived close: %q %q %q", got.Stage, got.StageOf, got.StageLabel)
			}
			if len(got.StageErrors) == 0 || !strings.Contains(got.StageErrors[len(got.StageErrors)-1], "cycle interrupted") {
				t.Fatalf("no interrupted-state stage error: %v", got.StageErrors)
			}

			// The finished row beside it: untouched.
			earlier, err := store.ListRunLogs(2)
			if err != nil {
				t.Fatalf("ListRunLogs: %v", err)
			}
			_ = done
			if len(earlier) != 2 {
				t.Fatalf("runlog has %d rows, want 2", len(earlier))
			}
			if earlier[1].ID == got.ID && earlier[1].Verdict != VerdictQuiet {
				t.Fatalf("finished row was overwritten: verdict %q", earlier[1].Verdict)
			}
			if earlier[1].FinishedAt != "2026-01-01T09:10:00Z" {
				t.Fatalf("finished row's end time changed: %q", earlier[1].FinishedAt)
			}
		})
	}
}

// TestCloseInterruptedRuns_closesOnlyOpenRows: a recovered row stays
// closed — a second sweep is a no-op, so repeated boots can't stack
// duplicated stage errors onto one crash.
func TestCloseInterruptedRuns_closesOnlyOpenRows(t *testing.T) {
	for name, store := range runRecoveryCases(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := store.AddRunLog(RunLog{StartedAt: "2026-01-01T10:00:00Z"}); err != nil {
				t.Fatalf("AddRunLog: %v", err)
			}
			now := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)
			if _, err := store.CloseInterruptedRuns(now); err != nil {
				t.Fatalf("first sweep: %v", err)
			}
			closed, err := store.CloseInterruptedRuns(now.Add(5 * time.Minute))
			if err != nil {
				t.Fatalf("second sweep: %v", err)
			}
			if closed != 0 {
				t.Fatalf("second sweep closed %d rows, want 0", closed)
			}
			last, _, _ := store.GetLastRun()
			if n := len(last.StageErrors); n != 1 {
				t.Fatalf("stage errors stacked on re-sweep: %d", n)
			}
		})
	}
}

// TestCloseInterruptedRuns_freshStore: an empty ledger sweeps clean —
// boot recovery never invents work.
func TestCloseInterruptedRuns_freshStore(t *testing.T) {
	for name, store := range runRecoveryCases(t) {
		t.Run(name, func(t *testing.T) {
			closed, err := store.CloseInterruptedRuns(time.Now())
			if err != nil {
				t.Fatalf("CloseInterruptedRuns: %v", err)
			}
			if closed != 0 {
				t.Fatalf("closed %d rows on a fresh store, want 0", closed)
			}
		})
	}
}
