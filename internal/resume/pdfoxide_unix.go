//go:build !windows

package resume

import (
	"fmt"
	"strings"

	pdfoxide "github.com/yfedoseev/pdf_oxide/go"
)

// extractPDFoxide is the internal seam for the purego pdf_oxide backend.
// Kept out of extract.go because pdf_oxide's Go bindings call purego.Dlopen,
// which only exists on unix-like platforms — windows/amd64|arm64 cannot
// compile this file (see pdfoxide_windows.go for the stub).
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
