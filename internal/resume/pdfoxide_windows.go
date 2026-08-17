//go:build windows

package resume

import "errors"

// extractPDFoxide is the internal seam for the purego pdf_oxide backend.
// pdf_oxide's Go bindings use purego.Dlopen (unix-only; purego's Windows API
// is LoadLibrary-based), so the backend cannot run on Windows. The poppler
// fallback covers Windows instead, so this stub always fails and Extract
// degrades to pdftotext.
var extractPDFoxide = func(path string) (string, int, error) {
	return "", 0, errors.New("pdf_oxide backend is not supported on Windows")
}
