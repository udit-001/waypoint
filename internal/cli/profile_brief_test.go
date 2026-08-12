package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// captureStdout runs fn with os.Stdout redirected to a buffer and returns
// what fn wrote to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	r.Close()
	return buf.String()
}

func TestProfileBriefJSON_empty(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = true

	out := captureStdout(t, func() {
		if err := profileBriefCmd.RunE(profileBriefCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var b map[string]any
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}

	open, _ := b["open"].([]any)
	if len(open) == 0 {
		t.Errorf("empty brief: expected open items, got %v", b["open"])
	}
	if b["complete"] != false {
		t.Errorf("empty brief: expected complete=false, got %v", b["complete"])
	}
}

func TestProfileBriefJSON_complete(t *testing.T) {
	fake := db.NewFakeStore()
	fake.UpsertProfile(map[string]any{
		"remote":              "hybrid",
		"location_preference": `["Bengaluru"]`,
		"companies":           `["Gojek"]`,
		"keywords":            `["kotlin"]`,
	})
	store = fake
	jsonOut = true

	out := captureStdout(t, func() {
		if err := profileBriefCmd.RunE(profileBriefCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var b map[string]any
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}

	if b["complete"] != true {
		t.Errorf("expected complete=true, got %v\n%s", b["complete"], out)
	}
	if arr, _ := b["open"].([]any); len(arr) != 0 {
		t.Errorf("expected empty open, got %v", b["open"])
	}
}

func TestProfileBriefTableCaps(t *testing.T) {
	fake := db.NewFakeStore()
	fake.UpsertProfile(map[string]any{
		"title":  "Bioinformatics Researcher",
		"remote": "remote",
	})
	store = fake
	jsonOut = false

	out := captureStdout(t, func() {
		if err := profileBriefCmd.RunE(profileBriefCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Bioinformatics Researcher") {
		t.Errorf("table output missing title: %q", out)
	}
	if !strings.Contains(out, "Status") {
		t.Errorf("table output missing status line: %q", out)
	}
}
