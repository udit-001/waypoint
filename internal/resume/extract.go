package resume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	pdfoxide "github.com/yfedoseev/pdf_oxide/go"
)

// Result is the full extract outcome: which backend produced the text,
// page/char counts, and the redaction report (embedded).
type Result struct {
	Backend string `json:"backend"` // "pdf_oxide" | "poppler"
	Pages   int    `json:"pages"`
	Chars   int    `json:"chars"`
	RedactResult
}

// DoctorReport reports what extraction backends are available.
type DoctorReport struct {
	LibPresent bool   `json:"lib_present"`
	LibPath    string `json:"lib_path"`
	Backend    string `json:"backend"` // "pdf_oxide" | "poppler" | "none"
	Next       string `json:"next"`
}

// extractPDFoxide is the internal seam for the purego pdf_oxide backend.
var extractPDFoxide = func(path string) (text string, pages int, err error) {
	doc, err := pdfoxide.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("pdf_oxide open: %w", err)
	}
	defer doc.Close()
	text, err = doc.ExtractAllText()
	if err != nil {
		return "", 0, fmt.Errorf("pdf_oxide extract: %w", err)
	}
	pages, err = doc.PageCount()
	if err != nil {
		return "", 0, fmt.Errorf("pdf_oxide page count: %w", err)
	}
	return strings.TrimSpace(text), pages, nil
}

// extractPoppler is the internal seam for the pdftotext fallback backend.
var extractPoppler = func(path string) (text string, pages int, err error) {
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	out, err := cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("pdftotext: %w", err)
	}
	// pdftotext separates pages with form feeds (^L)
	pages = strings.Count(string(out), "\x0c") + 1
	return strings.TrimSpace(string(out)), pages, nil
}

// popplerLookup checks for the pdftotext binary. Seam for tests.
var popplerLookup = exec.LookPath

// libManager gates the process-wide PDF_OXIDE_LIB_PATH env var. pdf_oxide
// dlopens the library exactly once per process, so the env must be set before
// the first call; setLibPathOnce is a seam for tests.
type libManager struct {
	once sync.Once
	err  error
}

func (m *libManager) setPathOnce(path string) {
	m.once.Do(func() {
		if err := os.Setenv("PDF_OXIDE_LIB_PATH", path); err != nil {
			m.err = fmt.Errorf("set PDF_OXIDE_LIB_PATH: %w", err)
		}
	})
}

var lib = &libManager{}

// Extract returns redacted text for a PDF resume, choosing a backend
// automatically: pdf_oxide when its native lib is (or can be) installed,
// otherwise the pdftotext fallback. Redaction always applies unless
// opts.NoRedact is set.
func Extract(ctx context.Context, path string, opts Options) (Result, error) {
	if fi, err := os.Stat(path); err != nil {
		return Result{}, fmt.Errorf("resume extract: %w", err)
	} else if fi.IsDir() {
		return Result{}, fmt.Errorf("resume extract: %s is a directory", path)
	}

	libErr := error(nil)
	if lp, err := ensureLib(libBase()); err == nil {
		lib.setPathOnce(lp)
		if lib.err != nil {
			return Result{}, lib.err
		}
		if text, pages, err := extractPDFoxide(path); err == nil {
			return finishExtract("pdf_oxide", text, pages, opts)
		} else {
			libErr = err
		}
	} else {
		libErr = err
	}

	if text, pages, err := extractPoppler(path); err == nil {
		return finishExtract("poppler", text, pages, opts)
	} else {
		return Result{}, fmt.Errorf(
			"resume extract: no backend available (pdf_oxide: %v; poppler: %w; run 'waypoint resume doctor')",
			libErr, err)
	}
}

func finishExtract(backend, text string, pages int, opts Options) (Result, error) {
	rr, err := Redact(text, opts)
	if err != nil {
		return Result{}, fmt.Errorf("resume redact: %w", err)
	}
	return Result{Backend: backend, Pages: pages, Chars: len(text), RedactResult: rr}, nil
}

// Doctor reports extraction backend availability.
func Doctor() DoctorReport {
	lp := libPath(libBase())
	_, statErr := os.Stat(lp)
	_, popplerErr := popplerLookup("pdftotext")

	backend := "none"
	next := "run 'waypoint resume extract <file.pdf>' to install the pdf_oxide library automatically; or install poppler-utils for the fallback backend"
	switch {
	case statErr == nil:
		backend = "pdf_oxide"
		next = "all backends ready"
	case popplerErr == nil:
		backend = "poppler"
		next = "pdf_oxide library not installed; 'waypoint resume extract' will fetch it on demand"
	}

	return DoctorReport{
		LibPresent: statErr == nil,
		LibPath:    lp,
		Backend:    backend,
		Next:       next,
	}
}

// errorsUnused keeps the errors import honest if poppler's classify path
// ever diverges; remove when unused.
var _ = errors.Is
