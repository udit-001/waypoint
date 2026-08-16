package resume

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/udit-001/waypoint/internal/config"
)

// libVersion is the pinned pdf_oxide release. The download URL, checksum
// sidecar, and archive layout all come from the same release.
const libVersion = "0.3.77"

// platformSuffix maps GOOS/GOARCH to the release asset suffix
// (mirrors pdf_oxide's own installer).
var platformSuffix = map[string]string{
	"linux/amd64":   "linux-amd64",
	"linux/arm64":   "linux-arm64",
	"darwin/amd64":  "darwin-amd64",
	"darwin/arm64":  "darwin-arm64",
	"windows/amd64": "windows-amd64",
}

// libBase resolves where waypoint keeps downloaded native libraries:
// <config dir>/lib. Seam for tests.
var libBase = func() string {
	return filepath.Join(configDir(), "lib")
}

// fetch downloads a URL into memory. Seam for tests.
var fetch = func(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec // pinned https URL
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<20))
}

// libName returns the shared-library filename for the current platform.
func libName() string {
	for osName, ext := range map[string]string{"linux": ".so", "darwin": ".dylib", "windows": ".dll"} {
		if runtime.GOOS == osName {
			if osName == "windows" {
				return "pdf_oxide.dll"
			}
			return "libpdf_oxide" + ext
		}
	}
	return "libpdf_oxide.so"
}

// goosDir returns the GOOS_GOARCH directory name inside the release archive.
func goosDir() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

// isAmd64Linux reports whether the platform is linux/amd64 (tests).
func isAmd64Linux() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
}

// libPath returns the expected on-disk location of the shared library.
func libPath(baseDir string) string {
	return filepath.Join(baseDir, "pdf_oxide", "v"+libVersion, "lib", goosDir(), libName())
}

// ensureLib makes sure the pdf_oxide shared library is installed under
// baseDir and returns its absolute path. An existing file short-circuits
// (no network). Downloads verify against the release's .sha256 sidecar and
// extract atomically-ish (temp dir + rename) so a failure leaves nothing.
func ensureLib(baseDir string) (string, error) {
	target := libPath(baseDir)
	if fi, err := os.Stat(target); err == nil && fi.Size() > 0 {
		return target, nil
	}
	suffix, ok := platformSuffix[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("pdf_oxide lib: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	asset := "pdf_oxide-go-ffi-shared-" + suffix + ".tar.gz"
	url := "https://github.com/yfedoseev/pdf_oxide/releases/download/v" + libVersion + "/" + asset

	tgz, err := fetch(url)
	if err != nil {
		return "", err
	}
	sidecar, err := fetch(url + ".sha256")
	if err != nil {
		return "", fmt.Errorf("fetch checksum: %w", err)
	}
	expected := strings.Fields(string(sidecar))
	if len(expected) == 0 || len(expected[0]) != 64 {
		return "", fmt.Errorf("invalid checksum sidecar for %s", asset)
	}
	got := sha256.Sum256(tgz)
	if hex.EncodeToString(got[:]) != expected[0] {
		return "", fmt.Errorf("checksum mismatch for %s", asset)
	}

	dst := filepath.Join(baseDir, "pdf_oxide", "v"+libVersion)
	if err := extractTarGz(tgz, dst); err != nil {
		return "", fmt.Errorf("extract %s: %w", asset, err)
	}
	return target, nil
}

// extractTarGz unpacks a pdf_oxide release archive under dst.
func extractTarGz(tgz []byte, dst string) error {
	gr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	// extract to a temp sibling then rename: a failed install leaves nothing
	tmp := dst + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Join(tmp, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(name, tmp+string(filepath.Separator)) {
			return fmt.Errorf("archive path escapes root: %s", hdr.Name)
		}
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return os.Rename(tmp, dst)
}

// configDir resolves the waypoint config dir.
var configDir = config.ConfigDir
