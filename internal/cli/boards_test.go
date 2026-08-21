package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
)

// setupBoardsTest points the config and data dirs at temp dirs, injects a
// fake store, and returns a cleanup.
func setupBoardsTest(t *testing.T) {
	t.Helper()
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	cleanup := config.SetConfigDirForTesting(cfgDir)
	t.Cleanup(cleanup)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("data_dir = \""+dataDir+"\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	store = db.NewFakeStore()
	jsonOut = false // persistent global; reset so tests don't leak --json
	t.Cleanup(func() { store = nil })
}

// runCmd captures os.Stdout (printJSON and fmt.Print bypass cobra's out
// buffer) and returns everything the command wrote.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rootCmd.SetOut(w)
	rootCmd.SetErr(w)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	w.Close()
	os.Stdout = old
	rootCmd.SetArgs(nil)
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String(), err
}

func TestBoardsAddRejectsUnknownProvider(t *testing.T) {
	setupBoardsTest(t)
	_, err := runCmd(t, "boards", "add", "acme", "--url", "https://example.com/careers")
	if err == nil || !strings.Contains(err.Error(), "no provider matched") {
		t.Fatalf("err = %v, want no-provider failure", err)
	}
	// Nothing saved on failure.
	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(bf.Boards) != 0 {
		t.Fatalf("failed add saved %d board(s)", len(bf.Boards))
	}
	_ = cfg
}

func TestBoardsLifecycle(t *testing.T) {
	setupBoardsTest(t)

	// Seed one board directly through the store API (add's verify path
	// needs the network; it is exercised by live verification).
	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	bf.Upsert(config.BoardEntry{Name: "slack", Company: "Slack", URL: "https://salesforce.wd12.myworkdayjobs.com/Slack/", Provider: "workday", Enabled: true})
	if err := config.SaveBoards(cfg, bf); err != nil {
		t.Fatal(err)
	}

	// list shows it.
	out, err := runCmd(t, "boards", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "slack") || !strings.Contains(out, "workday") {
		t.Fatalf("list output missing board: %q", out)
	}

	// disable → sweep skips it entirely.
	if _, err := runCmd(t, "boards", "disable", "slack"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	reloaded, _, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	if e := reloaded.Find("slack"); e == nil || e.Enabled {
		t.Fatal("disable did not persist")
	}

	// remove → gone.
	if _, err := runCmd(t, "boards", "remove", "slack"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := runCmd(t, "boards", "remove", "slack"); err == nil {
		t.Fatal("removing a missing board must fail")
	}
	reloaded, _, _ = loadBoardsStore()
	if len(reloaded.Boards) != 0 {
		t.Fatalf("remove left %d board(s)", len(reloaded.Boards))
	}
}

func TestBoardsSweepReportsUnmatchableBoard(t *testing.T) {
	setupBoardsTest(t)

	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	bf.Upsert(config.BoardEntry{Name: "acme", Company: "Acme", URL: "https://example.com/careers", Provider: "", Enabled: true})
	if err := config.SaveBoards(cfg, bf); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "boards", "sweep", "--json")
	if err != nil {
		t.Fatalf("sweep must not hard-fail on one bad board: %v", err)
	}
	for _, want := range []string{`"failed": 1`, "no provider", "acme"} {
		if !strings.Contains(out, want) {
			t.Fatalf("sweep json missing %q in:\n%s", want, out)
		}
	}
}

// TestBoardsSweepWritesState: a board attempt that fails (offline-testable:
// no provider claims the URL) still ends with its sweep state recorded,
// so the Companies page can diagnose it inline.
func TestBoardsSweepWritesState(t *testing.T) {
	setupBoardsTest(t)

	// Mirror `waypoint init`: migrations exist before any command runs.
	seedCfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	seedDB, err := db.Open(config.DBPath(seedCfg))
	if err != nil {
		t.Fatal(err)
	}
	if err := seedDB.RunMigrations(""); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedDB.Close()

	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	bf.Upsert(config.BoardEntry{Name: "acme", Company: "Acme", URL: "https://example.com/careers", Enabled: true})
	if err := config.SaveBoards(cfg, bf); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "boards", "sweep"); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// PersistentPreRunE opens the real database (data_dir points at the
	// test dir) — assert against it, not the injected fake.
	cfgLoaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s, err := db.Open(config.DBPath(cfgLoaded))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer s.Close()
	states, err := s.GetBoardSweepStates()
	if err != nil {
		t.Fatalf("GetBoardSweepStates: %v", err)
	}
	st, ok := states["acme"]
	if !ok {
		t.Fatalf("no sweep state recorded for acme; states = %v", states)
	}
	if st.OK || st.Error == "" {
		t.Errorf("state = %+v, want OK=false with a diagnostic", st)
	}
	if _, err := time.Parse(time.RFC3339, st.At); err != nil {
		t.Errorf("At = %q is not RFC3339: %v", st.At, err)
	}
}

func TestBoardsSweepNoBoards(t *testing.T) {
	setupBoardsTest(t)
	out, err := runCmd(t, "boards", "sweep")
	if err != nil {
		t.Fatalf("sweep with no boards: %v", err)
	}
	if !strings.Contains(out, "No enabled boards") {
		t.Fatalf("out = %q", out)
	}
}

func TestBoardsDetailUnknownBoard(t *testing.T) {
	setupBoardsTest(t)
	_, err := runCmd(t, "boards", "detail", "nope", "42")
	if err == nil || !strings.Contains(err.Error(), "no board named") {
		t.Fatalf("err = %v, want no-board failure", err)
	}
}
