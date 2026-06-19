package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SwatiBio/job-tracker/internal/version"
)

var updateForce bool

func init() {
	updateCmd.Flags().BoolVarP(&updateForce, "force", "f", false, "Reinstall even if already up to date")
	rootCmd.AddCommand(updateCmd)
}

const (
	ghOwner = "SwatiBio"
	ghRepo  = "job-tracker"
)

type ghRelease struct {
	TagName string `json:"tag_name"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update job-tracker to the latest version via 'go install'",
	Long: `Update job-tracker to the latest release by running:
  go install github.com/SwatiBio/job-tracker/cmd/job-tracker@latest

This compiles from source — no binary download, no Windows SmartScreen flags.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Printf("  Checking for updates...\n")

		rel, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to fetch latest release: %w", err)
		}

		latest := strings.TrimPrefix(rel.TagName, "v")
		fmt.Printf("  Latest version: %s\n", rel.TagName)

		if !updateForce {
			stored := readStoredVersion()
			if stored != "" && semverCompare(stored, latest) >= 0 {
				fmt.Printf("  Already up to date (v%s)\n", stored)
				fmt.Println()
				return nil
			}
		}

		goPath, err := exec.LookPath("go")
		if err != nil {
			fmt.Println()
			fmt.Println("  Go is not installed on your PATH.")
			fmt.Println("  Install manually with:")
			fmt.Printf("    go install github.com/%s/%s/cmd/job-tracker@%s\n", ghOwner, ghRepo, rel.TagName)
			fmt.Println()
			return nil
		}

		module := fmt.Sprintf("github.com/%s/%s/cmd/job-tracker@%s", ghOwner, ghRepo, rel.TagName)
		fmt.Printf("  Running: go install %s\n", module)

		c := exec.Command(goPath, "install", module)
		output, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go install failed: %w\n%s", err, string(output))
		}

		writeStoredVersion(latest)
		fmt.Printf("  Updated to %s\n", rel.TagName)
		fmt.Printf("  Restart the server to use the new version\n")
		fmt.Println()
		return nil
	},
}

func storedVersionFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".job-tracker", ".version")
}

func readStoredVersion() string {
	b, err := os.ReadFile(storedVersionFile())
	if err != nil {
		// fall back to the embedded version
		v := strings.TrimPrefix(version.Version, "v")
		if v == "" || v == "dev" {
			return ""
		}
		return v
	}
	return strings.TrimSpace(string(b))
}

func writeStoredVersion(v string) {
	_ = os.MkdirAll(filepath.Dir(storedVersionFile()), 0755)
	_ = os.WriteFile(storedVersionFile(), []byte(v+"\n"), 0644)
}

// semverCompare returns -1 if a < b, 0 if a == b, 1 if a > b.
func semverCompare(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	min := len(pa)
	if len(pb) < min {
		min = len(pb)
	}
	for i := 0; i < min; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	if len(pa) < len(pb) {
		return -1
	}
	if len(pa) > len(pb) {
		return 1
	}
	return 0
}

func parseSemver(v string) []int {
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nums
		}
		nums = append(nums, n)
	}
	return nums
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
