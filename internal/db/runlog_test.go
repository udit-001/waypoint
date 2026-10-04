package db

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// newRunLogStore opens a migrated SQLite store for runlog tests.
func newRunLogStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	raw := s.(*SQLiteStore)
	t.Cleanup(func() { raw.Close() })
	if err := raw.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return raw
}

func sampleRunLog() RunLog {
	return RunLog{
		Verdict:             VerdictWaitingOnYou,
		StartedAt:           "2026-10-05T01:00:00Z",
		FinishedAt:          "2026-10-05T01:00:41Z",
		DurationMs:          41000,
		PostingsNew:         12,
		PostingsShortlisted: 3,
		PostingsDismissed:   9,
		PostingsErrored:     0,
		PerSource: []SourceOutcome{
			{Source: "iisc", Scraped: 20, New: 5, Shortlisted: 1, Dismissed: 4},
			{Source: "linkedin", Scraped: 7, New: 7, Shortlisted: 2, Dismissed: 5},
			{Source: "indeed", Skipped: true, Reason: "disabled"},
		},
		StageErrors: []string{"sweep ncbs: context deadline exceeded"},
	}
}

// assertRunLogEqual compares the fields a round-trip must preserve.
func assertRunLogEqual(t *testing.T, got, want RunLog) {
	t.Helper()
	if got.Verdict != want.Verdict {
		t.Errorf("verdict = %q, want %q", got.Verdict, want.Verdict)
	}
	if got.StartedAt != want.StartedAt || got.FinishedAt != want.FinishedAt {
		t.Errorf("times = %q/%q, want %q/%q", got.StartedAt, got.FinishedAt, want.StartedAt, want.FinishedAt)
	}
	if got.DurationMs != want.DurationMs {
		t.Errorf("durationMs = %d, want %d", got.DurationMs, want.DurationMs)
	}
	if got.PostingsNew != want.PostingsNew || got.PostingsShortlisted != want.PostingsShortlisted ||
		got.PostingsDismissed != want.PostingsDismissed || got.PostingsErrored != want.PostingsErrored {
		t.Errorf("counts = %d/%d/%d/%d, want %d/%d/%d/%d",
			got.PostingsNew, got.PostingsShortlisted, got.PostingsDismissed, got.PostingsErrored,
			want.PostingsNew, want.PostingsShortlisted, want.PostingsDismissed, want.PostingsErrored)
	}
	if !reflect.DeepEqual(got.PerSource, want.PerSource) {
		t.Errorf("perSource = %+v, want %+v", got.PerSource, want.PerSource)
	}
	if !reflect.DeepEqual(got.StageErrors, want.StageErrors) {
		t.Errorf("stageErrors = %v, want %v", got.StageErrors, want.StageErrors)
	}
}

// TestRunLog_SQLiteRoundTrip: the verdict and the evidence stored under
// it survive the store — one log, two renderings.
func TestRunLog_SQLiteRoundTrip(t *testing.T) {
	s := newRunLogStore(t)
	want := sampleRunLog()

	id, err := s.AddRunLog(want)
	if err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id = %d, want > 0", id)
	}

	got, ok, err := s.GetLastRun()
	if err != nil || !ok {
		t.Fatalf("GetLastRun: ok=%v err=%v", ok, err)
	}
	assertRunLogEqual(t, got, want)
}

// TestRunLog_SQLiteListNewestFirst: ListRunLogs returns newest first and
// honours the limit.
func TestRunLog_SQLiteListNewestFirst(t *testing.T) {
	s := newRunLogStore(t)
	for _, v := range []Verdict{VerdictQuiet, VerdictNothingSurvived, VerdictDegraded} {
		entry := sampleRunLog()
		entry.Verdict = v
		if _, err := s.AddRunLog(entry); err != nil {
			t.Fatalf("AddRunLog(%s): %v", v, err)
		}
	}

	all, err := s.ListRunLogs(0)
	if err != nil {
		t.Fatalf("ListRunLogs: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].Verdict != VerdictDegraded || all[2].Verdict != VerdictQuiet {
		t.Errorf("order = %q..%q, want newest (degraded) first", all[0].Verdict, all[2].Verdict)
	}

	one, err := s.ListRunLogs(1)
	if err != nil {
		t.Fatalf("ListRunLogs(1): %v", err)
	}
	if len(one) != 1 || one[0].Verdict != VerdictDegraded {
		t.Errorf("limit 1 = %+v, want just the newest", one)
	}
}

// TestRunLog_SQLiteInFlightRow: a run opened but not yet closed is the
// "running now" signal — it must read back with an empty verdict and no
// finishedAt rather than failing the scan on NULL columns.
func TestRunLog_SQLiteInFlightRow(t *testing.T) {
	s := newRunLogStore(t)
	if _, err := s.AddRunLog(RunLog{StartedAt: "2026-10-05T01:00:00Z"}); err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}

	got, ok, err := s.GetLastRun()
	if err != nil || !ok {
		t.Fatalf("GetLastRun: ok=%v err=%v", ok, err)
	}
	if got.FinishedAt != "" {
		t.Errorf("finishedAt = %q, want empty", got.FinishedAt)
	}
	if got.Verdict != "" {
		t.Errorf("verdict = %q, want empty for an unfinished run", got.Verdict)
	}
	if len(got.PerSource) != 0 || len(got.StageErrors) != 0 {
		t.Errorf("evidence = %+v / %v, want empty", got.PerSource, got.StageErrors)
	}
}

// TestRunLog_UpdateClosesRow: closing a run writes the verdict and the
// evidence, and keeps the row's identity.
func TestRunLog_UpdateClosesRow(t *testing.T) {
	s := newRunLogStore(t)
	id, err := s.AddRunLog(RunLog{StartedAt: "2026-10-05T01:00:00Z"})
	if err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	want := sampleRunLog()
	if err := s.UpdateRunLog(id, want); err != nil {
		t.Fatalf("UpdateRunLog: %v", err)
	}
	got, ok, err := s.GetLastRun()
	if err != nil || !ok {
		t.Fatalf("GetLastRun: ok=%v err=%v", ok, err)
	}
	if got.ID != id {
		t.Errorf("id = %d, want %d", got.ID, id)
	}
	assertRunLogEqual(t, got, want)
}

// TestRunLog_FakeStoreParity: the fake store is the test seam every other
// package crosses — it must return the same record shape.
func TestRunLog_FakeStoreParity(t *testing.T) {
	f := NewFakeStore()
	if _, ok, _ := f.GetLastRun(); ok {
		t.Fatal("empty fake store should have no last run")
	}

	want := sampleRunLog()
	id, err := f.AddRunLog(want)
	if err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	got, ok, err := f.GetLastRun()
	if err != nil || !ok {
		t.Fatalf("GetLastRun: ok=%v err=%v", ok, err)
	}
	if got.ID != id {
		t.Errorf("id = %d, want %d", got.ID, id)
	}
	assertRunLogEqual(t, got, want)

	// An in-flight row reads back with no verdict.
	if _, err := f.AddRunLog(RunLog{StartedAt: "2026-10-05T02:00:00Z"}); err != nil {
		t.Fatalf("AddRunLog in-flight: %v", err)
	}
	last, _, _ := f.GetLastRun()
	if last.Verdict != "" || last.FinishedAt != "" {
		t.Errorf("in-flight row = %+v, want empty verdict/finishedAt", last)
	}

	// Newest first, limit honoured.
	runs, err := f.ListRunLogs(1)
	if err != nil {
		t.Fatalf("ListRunLogs: %v", err)
	}
	if len(runs) != 1 || runs[0].StartedAt != "2026-10-05T02:00:00Z" {
		t.Errorf("ListRunLogs(1) = %+v, want just the in-flight run", runs)
	}

	// Closing the row replaces it in place.
	closed := sampleRunLog()
	closed.Verdict = VerdictQuiet
	if err := f.UpdateRunLog(id, closed); err != nil {
		t.Fatalf("UpdateRunLog: %v", err)
	}
	runs, _ = f.ListRunLogs(10)
	var found bool
	for _, r := range runs {
		if r.ID == id {
			found = true
			if r.Verdict != VerdictQuiet {
				t.Errorf("closed verdict = %q, want quiet", r.Verdict)
			}
		}
	}
	if !found {
		t.Error("closed run not found in ListRunLogs")
	}
}

// TestRunLog_SQLiteListEmpty: an empty runlog lists as an empty array, not
// a nil slice — `runs --json` and both REST routes must say [] on a fresh
// install, never null.
func TestRunLog_SQLiteListEmpty(t *testing.T) {
	s := newRunLogStore(t)
	runs, err := s.ListRunLogs(0)
	if err != nil {
		t.Fatalf("ListRunLogs: %v", err)
	}
	if runs == nil {
		t.Fatal("ListRunLogs returned a nil slice; JSON would be null, not []")
	}
	if len(runs) != 0 {
		t.Errorf("len = %d, want 0", len(runs))
	}

	f := NewFakeStore()
	fakeRuns, err := f.ListRunLogs(0)
	if err != nil {
		t.Fatalf("fake ListRunLogs: %v", err)
	}
	if fakeRuns == nil || len(fakeRuns) != 0 {
		t.Errorf("fake ListRunLogs = %+v, want an empty non-nil slice", fakeRuns)
	}
}

// TestVerdict_IsDry pins the fallback trigger's definition: quiet and
// nothing-survived are dry; degraded and waiting-on-you never are.
func TestVerdict_IsDry(t *testing.T) {
	dry := map[Verdict]bool{
		VerdictQuiet:           true,
		VerdictNothingSurvived: true,
		VerdictWaitingOnYou:    false,
		VerdictDegraded:        false,
		Verdict(""):            false,
	}
	for v, want := range dry {
		if got := v.IsDry(); got != want {
			t.Errorf("%q.IsDry() = %v, want %v", v, got, want)
		}
	}
}

// TestCountPostings: the review-queue depth the verdict reads.
func TestCountPostings(t *testing.T) {
	s := newRunLogStore(t)
	if err := s.AddPostings([]scraper.Result{
		{URL: "https://a/1", Title: "A"},
		{URL: "https://a/2", Title: "B"},
	}); err != nil {
		t.Fatalf("AddPostings: %v", err)
	}
	if err := s.SetPostingStatus("https://a/2", StatusShortlisted); err != nil {
		t.Fatalf("SetPostingStatus: %v", err)
	}

	for _, tc := range []struct {
		status string
		want   int
	}{{"", 2}, {StatusNew, 1}, {StatusShortlisted, 1}, {StatusDismissed, 0}} {
		got, err := s.CountPostings(tc.status)
		if err != nil {
			t.Fatalf("CountPostings(%q): %v", tc.status, err)
		}
		if got != tc.want {
			t.Errorf("CountPostings(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

// TestPostings_SourcePersisted: a posting keeps the scraper it came from,
// so per-source outcomes stay attributable across cycles.
func TestPostings_SourcePersisted(t *testing.T) {
	s := newRunLogStore(t)
	if err := s.AddPostings([]scraper.Result{
		{URL: "https://a/1", Title: "A", Source: "iisc"},
	}); err != nil {
		t.Fatalf("AddPostings: %v", err)
	}
	got, ok, err := s.GetPosting("https://a/1")
	if err != nil || !ok {
		t.Fatalf("GetPosting: ok=%v err=%v", ok, err)
	}
	if got.Result.Source != "iisc" {
		t.Errorf("source = %q, want iisc", got.Result.Source)
	}

	f := NewFakeStore()
	if err := f.AddPostings([]scraper.Result{{URL: "https://a/1", Title: "A", Source: "iisc"}}); err != nil {
		t.Fatalf("fake AddPostings: %v", err)
	}
	fp, _, _ := f.GetPosting("https://a/1")
	if fp.Result.Source != "iisc" {
		t.Errorf("fake source = %q, want iisc", fp.Result.Source)
	}
}
