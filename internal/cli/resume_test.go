package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Slice: extract on a missing file emits a JSON error envelope, exit path.
func TestResumeExtract_MissingFileJSONError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.pdf")
	out := captureStdout(t, func() {
		_ = resumeExtractCmd.RunE(resumeExtractCmd, []string{missing})
	})
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if env.OK {
		t.Error("ok = true on failure, want false")
	}
	if !strings.Contains(env.Error, "no such file") {
		t.Errorf("error = %q, want file-missing message", env.Error)
	}
}

// Slice: doctor always emits a JSON envelope with the report keys.
func TestResumeDoctor_JSONShape(t *testing.T) {
	out := captureStdout(t, func() {
		if err := resumeDoctorCmd.RunE(resumeDoctorCmd, nil); err != nil {
			t.Fatalf("doctor RunE: %v", err)
		}
	})
	var env struct {
		OK         bool   `json:"ok"`
		LibPresent bool   `json:"lib_present"`
		LibPath    string `json:"lib_path"`
		Backend    string `json:"backend"`
		Next       string `json:"next"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if !env.OK {
		t.Error("ok = false, want true")
	}
	if env.LibPath == "" || env.Next == "" {
		t.Errorf("incomplete report: %+v", env)
	}
}

// Slice: extract on a missing file returns non-nil so Execute exits 1.
func TestResumeExtract_ReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.pdf")
	if err := resumeExtractCmd.RunE(resumeExtractCmd, []string{missing}); err == nil {
		t.Fatal("RunE returned nil on missing file")
	}
}
