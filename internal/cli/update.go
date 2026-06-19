package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	ghOwner = "SwatiBio"
	ghRepo  = "job-tracker"
)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update job-tracker to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Printf("  Checking for updates...\n")

		rel, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to fetch latest release: %w", err)
		}

		fmt.Printf("  Latest version: %s\n", rel.TagName)

		ext := ".tar.gz"
		if runtime.GOOS == "windows" {
			ext = ".zip"
		}
		suffix := fmt.Sprintf("_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)

		var asset *ghAsset
		for _, a := range rel.Assets {
			if strings.HasSuffix(a.Name, suffix) {
				asset = &a
				break
			}
		}
		if asset == nil {
			return fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		selfPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot determine executable path: %w", err)
		}
		selfPath, err = filepath.EvalSymlinks(selfPath)
		if err != nil {
			return fmt.Errorf("cannot resolve executable path: %w", err)
		}

		fmt.Printf("  Downloading %s...\n", asset.Name)
		tmpDir, err := os.MkdirTemp("", "job-tracker-update")
		if err != nil {
			return fmt.Errorf("cannot create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		archivePath := filepath.Join(tmpDir, asset.Name)
		if err := downloadFile(archivePath, asset.DownloadURL); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		fmt.Printf("  Extracting...\n")
		binPath, err := extractBinary(archivePath, tmpDir)
		if err != nil {
			return fmt.Errorf("extraction failed: %w", err)
		}

		if err := os.Rename(binPath, selfPath); err != nil {
			// Fallback: copy + remove
			if err := copyFile(selfPath, binPath); err != nil {
				return fmt.Errorf("failed to replace binary: %w", err)
			}
		}

		fmt.Printf("  Updated to %s\n", rel.TagName)
		fmt.Println()
		return nil
	},
}

func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", ghOwner, ghRepo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func downloadFile(path, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(archivePath, tmpDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, tmpDir)
	}
	return extractTarGz(archivePath, tmpDir)
}

func extractTarGz(archivePath, tmpDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Base(hdr.Name)
		if name != "job-tracker" && name != "job-tracker.exe" {
			continue
		}
		outPath := filepath.Join(tmpDir, name)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		defer out.Close()
		if _, err := io.Copy(out, tr); err != nil {
			return "", err
		}
		if err := out.Chmod(0755); err != nil {
			return "", err
		}
		return outPath, nil
	}
	return "", fmt.Errorf("binary not found in archive")
}

func extractZip(archivePath, tmpDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name != "job-tracker" && name != "job-tracker.exe" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		outPath := filepath.Join(tmpDir, name)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		defer out.Close()
		if _, err := io.Copy(out, rc); err != nil {
			return "", err
		}
		_ = out.Chmod(0755)
		return outPath, nil
	}
	return "", fmt.Errorf("binary not found in archive")
}

func copyFile(dst, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
