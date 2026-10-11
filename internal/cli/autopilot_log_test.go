package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/autopilot"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// The capture tests run the real redirect and restore the process-global
// handles afterward — os.Stdout/os.Stderr and the default logger are
// package globals, so the save/restore is the test's own seam discipline.

// redirectForTest swaps the globals through captureDaemonOutput and hands
// back a restore func.
func redirectForTest(t *testing.T, path string) {
	t.Helper()
	f, err := captureDaemonOutput(path)
	if err != nil {
		t.Fatalf("captureDaemonOutput: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	out, errOut := os.Stdout, os.Stderr
	logWriter := log.Writer()
	t.Cleanup(func() {
		os.Stdout, os.Stderr = out, errOut
		log.SetOutput(logWriter)
	})
}

// TestCaptureDaemonOutput_appendsEverything: both handles land in the file,
// and a second capture appends rather than truncating — the raw appendix
// contract (WP-176).
func TestCaptureDaemonOutput_appendsEverything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autopilot.log")

	redirectForTest(t, path)
	fmt.Fprintln(os.Stdout, "stdout-line")
	fmt.Fprintln(os.Stderr, "stderr-line")
	log.Printf("logger-line")

	// Second daemon lifetime: append, not truncate.
	redirectForTest(t, path)
	log.Printf("second-lifetime")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(raw)
	for _, want := range []string{"stdout-line", "stderr-line", "logger-line", "second-lifetime"} {
		if !strings.Contains(got, want) {
			t.Errorf("log missing %q:\n%s", want, got)
		}
	}
	if len(strings.Split(strings.TrimSpace(got), "\n")) != 4 {
		t.Errorf("expected 4 lines (append, no truncate), got:\n%s", got)
	}
}

// TestCaptureDaemonOutput_cycleLeavesTraceableLines: the acceptance
// criterion at the real seam — a cycle run with the capture active leaves
// its stage/error lines in the file. The cycle is shaped to stay offline:
// discovery's trigger state is pre-seeded (no Exa calls) and the one
// source fails, which is the error path a raw log exists for.
func TestCaptureDaemonOutput_cycleLeavesTraceableLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autopilot.log")
	redirectForTest(t, path)

	f := db.NewFakeStore()
	// Seed the discovery trigger: one existing candidate plus a last-run
	// record — the cycle must not re-run discovery (and its live
	// enumeration). Candidates==0 would force first-run regardless.
	if err := f.SaveCandidates([]db.CompanyCandidate{{Name: "Seed Co", Status: "added"}}); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	if err := f.SaveDiscoveryLastRun(discovery.BriefHash(db.BriefFromProfile(db.Profile{}).LLMInput().Text()), time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("seed discovery state: %v", err)
	}
	cycle := autopilot.CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{&cliFailingScraper{name: "fixture"}},
		Recency:  14,
	}
	autopilot.Run(context.Background(), cycle)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "autopilot: sweep fixture failed") {
		t.Errorf("cycle's stage lines missing from the log:\n%s", got)
	}
}

// cliFailingScraper is a source whose sweep always fails — the error path
// the raw log exists to capture.
type cliFailingScraper struct{ name string }

func (s *cliFailingScraper) Name() string   { return s.name }
func (s *cliFailingScraper) Source() string { return s.name }
func (s *cliFailingScraper) Categories() []string {
	return []string{"test", "aggregator"} // aggregator: the selection fallback sweeps it
}
func (s *cliFailingScraper) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	return nil, fmt.Errorf("portal down")
}
