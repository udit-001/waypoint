package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// WP-178: `waypoint autopilot runs` exposes the runlog — verdict plus the
// per-source evidence under it. An agent must be able to name one of the
// five answers from --json alone.

func seedRuns(t *testing.T, f *db.FakeStore) {
	t.Helper()
	if _, err := f.AddRunLog(db.RunLog{
		Verdict:             db.VerdictWaitingOnYou,
		StartedAt:           "2026-10-05T01:00:00Z",
		FinishedAt:          "2026-10-05T01:00:41Z",
		DurationMs:          41000,
		PostingsNew:         12,
		PostingsShortlisted: 3,
		PostingsDismissed:   9,
		PerSource: []db.SourceOutcome{
			{Source: "iisc", Scraped: 20, New: 5, Shortlisted: 1, Dismissed: 4},
			{Source: "linkedin", Skipped: true, Reason: "disabled"},
		},
	}); err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	if _, err := f.AddRunLog(db.RunLog{
		Verdict:           db.VerdictDegraded,
		StartedAt:         "2026-10-05T07:00:00Z",
		FinishedAt:        "2026-10-05T07:00:03Z",
		DurationMs:        3000,
		StageErrors:       []string{"sweep ncbs: context deadline exceeded"},
		PostingsErrored:   1,
		PostingsDismissed: 2,
	}); err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
}

func TestAutopilotRunsJSON(t *testing.T) {
	f := db.NewFakeStore()
	seedRuns(t, f)
	store = f
	jsonOut = true
	defer func() { jsonOut = false }()

	out := captureStdout(t, func() {
		if err := autopilotRunsCmd.RunE(autopilotRunsCmd, nil); err != nil {
			t.Fatalf("runs: %v", err)
		}
	})

	var got struct {
		Runs []db.RunLog `json:"runs"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(got.Runs))
	}
	// Newest first.
	if got.Runs[0].Verdict != db.VerdictDegraded || got.Runs[1].Verdict != db.VerdictWaitingOnYou {
		t.Errorf("order = %q,%q, want degraded then waiting-on-you",
			got.Runs[0].Verdict, got.Runs[1].Verdict)
	}
	// The evidence rides with the verdict.
	last := got.Runs[1]
	if len(last.PerSource) != 2 || last.PerSource[0].Source != "iisc" || last.PerSource[0].Scraped != 20 {
		t.Errorf("perSource = %+v, want the iisc row", last.PerSource)
	}
	if !last.PerSource[1].Skipped || last.PerSource[1].Reason != "disabled" {
		t.Errorf("skipped row = %+v, want skipped/disabled", last.PerSource[1])
	}
	if len(got.Runs[0].StageErrors) != 1 {
		t.Errorf("stageErrors = %v, want the sweep failure", got.Runs[0].StageErrors)
	}
}

func TestAutopilotRunsJSONEmpty(t *testing.T) {
	store = db.NewFakeStore()
	jsonOut = true
	defer func() { jsonOut = false }()

	out := captureStdout(t, func() {
		if err := autopilotRunsCmd.RunE(autopilotRunsCmd, nil); err != nil {
			t.Fatalf("runs: %v", err)
		}
	})

	if !strings.Contains(out, `"runs": []`) {
		t.Errorf("empty runlog JSON = %q, want an empty runs array", out)
	}
}

func TestAutopilotRunsHuman(t *testing.T) {
	f := db.NewFakeStore()
	seedRuns(t, f)
	store = f
	jsonOut = false

	out := captureStdout(t, func() {
		if err := autopilotRunsCmd.RunE(autopilotRunsCmd, nil); err != nil {
			t.Fatalf("runs: %v", err)
		}
	})

	for _, want := range []string{
		"degraded",
		"waiting-on-you",
		"iisc",
		"scraped 20",
		"linkedin",
		"skipped",
		"disabled",
		"sweep ncbs: context deadline exceeded",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}
}

func TestAutopilotRunsLimit(t *testing.T) {
	f := db.NewFakeStore()
	seedRuns(t, f)
	store = f
	jsonOut = true

	old := autopilotRunsFlags.limit
	autopilotRunsFlags.limit = 1
	defer func() { autopilotRunsFlags.limit = old; jsonOut = false }()

	out := captureStdout(t, func() {
		if err := autopilotRunsCmd.RunE(autopilotRunsCmd, nil); err != nil {
			t.Fatalf("runs: %v", err)
		}
	})

	var got struct {
		Runs []db.RunLog `json:"runs"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got.Runs) != 1 || got.Runs[0].Verdict != db.VerdictDegraded {
		t.Errorf("limit 1 = %+v, want just the newest run", got.Runs)
	}
}
