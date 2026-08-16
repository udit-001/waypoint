package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoardsFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{DataDir: dir}

	// Missing file → empty store, no error.
	bf, err := LoadBoards(cfg)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(bf.Boards) != 0 {
		t.Fatalf("expected empty, got %d", len(bf.Boards))
	}

	bf.Upsert(BoardEntry{Name: "slack", Company: "Slack", URL: "https://salesforce.wd12.myworkdayjobs.com/Slack/", Provider: "workday", Enabled: true, AddedAt: "2026-08-16T00:00:00Z"})
	bf.Upsert(BoardEntry{Name: "khanacademy", Company: "Khan Academy", URL: "https://job-boards.greenhouse.io/khanacademy/", Provider: "greenhouse", Enabled: true})
	if err := SaveBoards(cfg, bf); err != nil {
		t.Fatalf("save: %v", err)
	}

	// File exists in data_dir, not config dir.
	if _, err := os.Stat(filepath.Join(dir, "boards.toml")); err != nil {
		t.Fatalf("boards.toml not in data_dir: %v", err)
	}

	got, err := LoadBoards(cfg)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.Boards) != 2 {
		t.Fatalf("reload count = %d", len(got.Boards))
	}
	// Sorted by name for stable diffs.
	if got.Boards[0].Name != "khanacademy" || got.Boards[1].Name != "slack" {
		t.Fatalf("order = %s, %s", got.Boards[0].Name, got.Boards[1].Name)
	}
	if got.Boards[1].Provider != "workday" || !got.Boards[1].Enabled {
		t.Fatalf("round-trip lost fields: %+v", got.Boards[1])
	}
}

func TestBoardsUpsertReplacesByName(t *testing.T) {
	bf := &BoardsFile{}
	bf.Upsert(BoardEntry{Name: "slack", URL: "a", Enabled: true})
	bf.Upsert(BoardEntry{Name: "slack", URL: "b", Enabled: false})
	if len(bf.Boards) != 1 {
		t.Fatalf("upsert appended: %d entries", len(bf.Boards))
	}
	if bf.Boards[0].URL != "b" || bf.Boards[0].Enabled {
		t.Fatalf("upsert did not replace: %+v", bf.Boards[0])
	}
}

func TestBoardsFindAndRemove(t *testing.T) {
	bf := &BoardsFile{}
	bf.Upsert(BoardEntry{Name: "a", Enabled: true})
	bf.Upsert(BoardEntry{Name: "b", Enabled: true})

	if bf.Find("b") == nil || bf.Find("nope") != nil {
		t.Fatal("Find misbehaves")
	}
	if !bf.Remove("a") || len(bf.Boards) != 1 || bf.Boards[0].Name != "b" {
		t.Fatal("Remove misbehaves")
	}
	if bf.Remove("a") {
		t.Fatal("Remove of missing returned true")
	}
}

func TestBoardsPathFallsBackToDefault(t *testing.T) {
	// nil cfg → default data dir (~/.waypoint), like DBPath.
	if got := BoardsPath(nil); got != filepath.Join(DefaultDataDir(), "boards.toml") {
		t.Fatalf("BoardsPath(nil) = %s", got)
	}
	// Empty DataDir behaves like nil (Config zero value).
	if got := BoardsPath(&Config{}); got != BoardsPath(nil) {
		t.Fatalf("BoardsPath(empty) = %s", got)
	}
}
