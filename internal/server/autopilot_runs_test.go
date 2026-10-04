package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// WP-178: the REST routes expose the same runlog records the CLI renders.

func seedAutopilotRuns(t *testing.T, f *db.FakeStore) {
	t.Helper()
	if _, err := f.AddRunLog(db.RunLog{
		Verdict:             db.VerdictWaitingOnYou,
		StartedAt:           "2026-10-05T01:00:00Z",
		FinishedAt:          "2026-10-05T01:00:41Z",
		DurationMs:          41000,
		PostingsNew:         12,
		PostingsShortlisted: 3,
		PerSource: []db.SourceOutcome{
			{Source: "iisc", Scraped: 20, New: 5, Shortlisted: 1, Dismissed: 4},
		},
	}); err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	if _, err := f.AddRunLog(db.RunLog{
		Verdict:     db.VerdictDegraded,
		StartedAt:   "2026-10-05T07:00:00Z",
		FinishedAt:  "2026-10-05T07:00:03Z",
		DurationMs:  3000,
		StageErrors: []string{"sweep ncbs: context deadline exceeded"},
		PostingsNew: 0,
		PerSource:   []db.SourceOutcome{{Source: "ncbs", Errored: 1}},
	}); err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
}

func TestAutopilotRunsRoute(t *testing.T) {
	f := db.NewFakeStore()
	seedAutopilotRuns(t, f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/autopilot/runs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}
	var got struct {
		Runs []db.RunLog `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", w.Body.String(), err)
	}
	if len(got.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(got.Runs))
	}
	if got.Runs[0].Verdict != db.VerdictDegraded || got.Runs[1].Verdict != db.VerdictWaitingOnYou {
		t.Errorf("order = %q,%q, want newest (degraded) first",
			got.Runs[0].Verdict, got.Runs[1].Verdict)
	}
	if len(got.Runs[1].PerSource) != 1 || got.Runs[1].PerSource[0].Scraped != 20 {
		t.Errorf("perSource = %+v, want the iisc row", got.Runs[1].PerSource)
	}
	if len(got.Runs[0].StageErrors) != 1 {
		t.Errorf("stageErrors = %v, want the sweep failure", got.Runs[0].StageErrors)
	}
}

func TestAutopilotRunsRouteLimit(t *testing.T) {
	f := db.NewFakeStore()
	seedAutopilotRuns(t, f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/autopilot/runs?limit=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got struct {
		Runs []db.RunLog `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", w.Body.String(), err)
	}
	if len(got.Runs) != 1 {
		t.Errorf("runs = %d, want 1", len(got.Runs))
	}
}

func TestAutopilotRunsRouteBadLimit(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/autopilot/runs?limit=nope", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400 for a malformed limit", w.Code)
	}
}

// TestAutopilotPayloadCarriesRuns: the web's single autopilot fetch gets
// the last-run hero and the recent-runs list from one record set, so a
// verdict line and a runs list never come from two sources of truth.
func TestAutopilotPayloadCarriesRuns(t *testing.T) {
	f := db.NewFakeStore()
	seedAutopilotRuns(t, f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/autopilot", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}
	var got struct {
		LastRun *db.RunLog  `json:"lastRun"`
		Runs    []db.RunLog `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", w.Body.String(), err)
	}
	if got.LastRun == nil || got.LastRun.Verdict != db.VerdictDegraded {
		t.Errorf("lastRun = %+v, want the newest run with its verdict", got.LastRun)
	}
	if len(got.Runs) != 2 {
		t.Errorf("runs = %d, want 2", len(got.Runs))
	}
}
