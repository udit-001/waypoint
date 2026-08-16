package resume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// touch creates an empty file in a temp dir and returns its path.
func touch(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("junk"), 0o644); err != nil {
		t.Fatalf("touch: %v", err)
	}
	return p
}

// fakeLibDir installs a fake pdf_oxide lib and points libBase at it.
func fakeLibDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lib")
	so := filepath.Join(dir, "pdf_oxide", "v"+libVersion, "lib", goosDir())
	if err := os.MkdirAll(so, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(so, libName()), []byte("FAKE"), 0o755); err != nil {
		t.Fatalf("write lib: %v", err)
	}
	old := libBase
	libBase = func() string { return dir }
	t.Cleanup(func() { libBase = old })
	return dir
}

// Slice: pdf_oxide backend when the lib is installed; redaction still applies.
func TestExtract_UsesPDFOxideAndRedacts(t *testing.T) {
	fakeLibDir(t)
	oldOxide := extractPDFoxide
	extractPDFoxide = func(path string) (string, int, error) {
		return "Jake Ryan\njake@su.edu", 2, nil
	}
	defer func() { extractPDFoxide = oldOxide }()
	popplerCalled := false
	oldPoppler := extractPoppler
	extractPoppler = func(string) (string, int, error) {
		popplerCalled = true
		return "", 0, nil
	}
	defer func() { extractPoppler = oldPoppler }()

	got, err := Extract(context.Background(), touch(t, "whatever.pdf"), Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got.Backend != "pdf_oxide" {
		t.Errorf("Backend = %q, want pdf_oxide", got.Backend)
	}
	if got.Pages != 2 {
		t.Errorf("Pages = %d, want 2", got.Pages)
	}
	if !got.EmailRedacted || !got.Redacted {
		t.Errorf("redaction flags = redacted:%v email:%v, want both true", got.Redacted, got.EmailRedacted)
	}
	if got.Text != "Jake Ryan\n[REDACTED]" {
		t.Errorf("Text = %q", got.Text)
	}
	if popplerCalled {
		t.Error("poppler called although pdf_oxide succeeded")
	}
}

// Slice: poppler fallback when the lib cannot be obtained (offline).
func TestExtract_FallsBackToPoppler(t *testing.T) {
	dir := t.TempDir()
	oldLibBase := libBase
	libBase = func() string { return filepath.Join(dir, "lib") }
	defer func() { libBase = oldLibBase }()
	oldFetch := fetch
	fetch = func(string) ([]byte, error) { return nil, errors.New("offline") }
	defer func() { fetch = oldFetch }()
	oldPoppler := extractPoppler
	extractPoppler = func(path string) (string, int, error) {
		return "Stephen Xu\n512-555-1234", 1, nil
	}
	defer func() { extractPoppler = oldPoppler }()

	got, err := Extract(context.Background(), touch(t, "x.pdf"), Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got.Backend != "poppler" {
		t.Errorf("Backend = %q, want poppler", got.Backend)
	}
	if !got.PhoneRedacted {
		t.Error("PhoneRedacted = false, want true (fallback output must still redact)")
	}
	if got.Text != "Stephen Xu\n[REDACTED]" {
		t.Errorf("Text = %q", got.Text)
	}
}

// Slice: no backend available → clear error naming both.
func TestExtract_NoBackend(t *testing.T) {
	oldFetch := fetch
	fetch = func(string) ([]byte, error) { return nil, errors.New("offline") }
	defer func() { fetch = oldFetch }()
	oldPoppler := extractPoppler
	extractPoppler = func(string) (string, int, error) { return "", 0, errors.New("pdftotext missing") }
	defer func() { extractPoppler = oldPoppler }()

	_, err := Extract(context.Background(), touch(t, "x.pdf"), Options{})
	if err == nil {
		t.Fatal("Extract succeeded with no backend, want error")
	}
}

// Slice: NoRedact bypasses the engine entirely.
func TestExtract_NoRedact(t *testing.T) {
	fakeLibDir(t)
	oldOxide := extractPDFoxide
	extractPDFoxide = func(path string) (string, int, error) { return "jake@su.edu", 1, nil }
	defer func() { extractPDFoxide = oldOxide }()

	got, err := Extract(context.Background(), touch(t, "x.pdf"), Options{NoRedact: true})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got.Text != "jake@su.edu" {
		t.Errorf("Text = %q, want raw", got.Text)
	}
	if got.Redacted {
		t.Error("Redacted = true with NoRedact")
	}
}

// Slice: missing input path errors before any backend runs.
func TestExtract_BadPath(t *testing.T) {
	oxideCalled := false
	oldOxide := extractPDFoxide
	extractPDFoxide = func(string) (string, int, error) {
		oxideCalled = true
		return "", 0, nil
	}
	defer func() { extractPDFoxide = oldOxide }()

	_, err := Extract(context.Background(), filepath.Join(t.TempDir(), "nope.pdf"), Options{})
	if err == nil {
		t.Fatal("Extract on missing file succeeded")
	}
	if oxideCalled {
		t.Error("backend called before path validation")
	}
}

// Slice: Doctor reports lib present / backend availability.
func TestDoctor_States(t *testing.T) {
	// lib present
	fakeLibDir(t)
	d := Doctor()
	if !d.LibPresent || d.Backend != "pdf_oxide" {
		t.Errorf("with lib: %+v, want lib_present + pdf_oxide", d)
	}

	// lib absent, poppler present
	libBase = func() string { return filepath.Join(t.TempDir(), "lib") }
	oldLookup := popplerLookup
	popplerLookup = func(string) (string, error) { return "/usr/bin/pdftotext", nil }
	defer func() { popplerLookup = oldLookup }()
	d = Doctor()
	if d.LibPresent || d.Backend != "poppler" {
		t.Errorf("without lib, with poppler: %+v", d)
	}

	// both absent
	popplerLookup = func(string) (string, error) { return "", errors.New("not found") }
	d = Doctor()
	if d.LibPresent || d.Backend != "none" {
		t.Errorf("neither: %+v", d)
	}
}
