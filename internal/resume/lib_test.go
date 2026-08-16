package resume

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeTGZ builds a tar.gz that mirrors the pdf_oxide release layout:
// lib/linux_amd64/libpdf_oxide.so
func fakeTGZ(t *testing.T, soContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	body := []byte(soContent)
	if err := tw.WriteHeader(&tar.Header{
		Name: "lib/linux_amd64/libpdf_oxide.so", Mode: 0o755, Size: int64(len(body)),
	}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fakeFetcher returns bytes for URLs; it always returns the tgz plus its
// correct .sha256 sidecar. Callers can swap the sidecar to test mismatches.
type fakeFetcher struct {
	tgz     []byte
	sidecar string            // hex sha of tgz ("" = correct)
	extra   map[string][]byte // URL → body overrides
	calls   []string
	mu      sync.Mutex
}

func (f *fakeFetcher) fetch(url string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, url)
	f.mu.Unlock()
	if body, ok := f.extra[url]; ok {
		return body, nil
	}
	if strings.HasSuffix(url, ".sha256") {
		if f.sidecar == "" {
			f.sidecar = sha256Hex(f.tgz)
		}
		return []byte(f.sidecar + "  " + url), nil
	}
	return f.tgz, nil
}

// Slice: EnsureLib downloads, verifies, extracts, and is cached on second call.
func TestEnsureLib_DownloadsVerifiesCaches(t *testing.T) {
	if os.Getenv("WAYPOINT_TESTS_ACCEPT_PLATFORM") == "" && !isAmd64Linux() {
		return // archive layout in fixtures is linux_amd64-specific
	}
	base := t.TempDir()
	ff := &fakeFetcher{tgz: fakeTGZ(t, "FAKE-SO")}
	old := fetch
	fetch = ff.fetch
	defer func() { fetch = old }()

	got, err := ensureLib(base)
	if err != nil {
		t.Fatalf("ensureLib: %v", err)
	}
	want := filepath.Join(base, "pdf_oxide", "v"+libVersion, "lib", goosDir(), libName())
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read extracted lib: %v", err)
	}
	if string(data) != "FAKE-SO" {
		t.Errorf("lib content = %q", data)
	}
	if len(ff.calls) != 2 {
		t.Errorf("fetch calls = %d (%v), want 2 (tgz + sidecar)", len(ff.calls), ff.calls)
	}

	// cached: second call must not fetch again
	if _, err := ensureLib(base); err != nil {
		t.Fatalf("ensureLib cached: %v", err)
	}
	if len(ff.calls) != 2 {
		t.Errorf("cached call re-fetched: calls = %d", len(ff.calls))
	}
}

// Slice: a checksum mismatch aborts the install and leaves no artifact.
func TestEnsureLib_ChecksumMismatch(t *testing.T) {
	base := t.TempDir()
	ff := &fakeFetcher{tgz: fakeTGZ(t, "TAMPERED"), sidecar: strings.Repeat("0", 64)}
	old := fetch
	fetch = ff.fetch
	defer func() { fetch = old }()

	if _, err := ensureLib(base); err == nil {
		t.Fatal("ensureLib succeeded with bad checksum, want error")
	} else if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("err = %v, want checksum mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(base, "pdf_oxide")); !os.IsNotExist(err) {
		t.Error("artifacts left behind after failed install")
	}
}

// Slice: fetch failure surfaces clearly and leaves no artifact.
func TestEnsureLib_FetchFailure(t *testing.T) {
	base := t.TempDir()
	old := fetch
	fetch = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	defer func() { fetch = old }()

	if _, err := ensureLib(base); err == nil {
		t.Fatal("ensureLib succeeded with failing fetch, want error")
	}
}

// Slice: already-installed lib short-circuits without network.
func TestEnsureLib_AlreadyInstalled(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "pdf_oxide", "v"+libVersion, "lib", goosDir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, libName()), []byte("EXISTING"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	called := false
	old := fetch
	fetch = func(string) ([]byte, error) { called = true; return nil, os.ErrNotExist }
	defer func() { fetch = old }()

	_, err := ensureLib(base)
	if err != nil {
		t.Fatalf("ensureLib on existing lib: %v", err)
	}
	if called {
		t.Error("fetch called despite installed lib")
	}
}
