package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SwatiBio/waypoint/internal/version"
)

var upgradeForce bool

func init() {
	upgradeCmd.Flags().BoolVarP(&upgradeForce, "force", "f", false, "Reinstall even if already up to date")
	upgradeCmd.Flags().Bool("no-skills", false, "Skip the skill upgrade prompt")
	rootCmd.AddCommand(upgradeCmd)
}

const (
	ghOwner = "SwatiBio"
	ghRepo  = "waypoint"
)

type ghRelease struct {
	TagName string `json:"tag_name"`
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade waypoint to the latest version via 'go install'",
	Long: `Upgrade waypoint to the latest release by running:
  go install github.com/SwatiBio/waypoint/cmd/waypoint@latest

This compiles from source — no binary download, no Windows SmartScreen flags.

If the server is running, it will be stopped before the upgrade and
restarted afterwards.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Printf("  Checking for upgrades...\n")

		rel, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to fetch latest release: %w", err)
		}

		latest := strings.TrimPrefix(rel.TagName, "v")
		current := strings.TrimPrefix(version.Version, "v")
		fmt.Printf("  Latest version: %s\n", rel.TagName)

		if !upgradeForce && current != "" && current != "dev" && semverCompare(current, latest) >= 0 {
			fmt.Printf("  Already up to date (v%s)\n", current)
			fmt.Println()
			return nil
		}

		// Stop server if running — the binary can't be replaced while
		// the process is alive, and the DB lock will conflict on restart.
		var pid int
		if data, err := os.ReadFile(pidFilePath()); err == nil {
			if p, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				pid = p
			}
		}

		if pid > 0 {
			fmt.Printf("  Stopping server (PID %d)...\n", pid)
			if err := killProcess(pid); err != nil {
				fmt.Printf("  Warning: could not stop server: %v\n", err)
				fmt.Printf("  Please stop it manually and re-run upgrade.\n")
				fmt.Println()
				return nil
			}
			_ = os.Remove(pidFilePath())
			fmt.Printf("  Server stopped.\n")
		}

		goPath, err := exec.LookPath("go")
		if err != nil {
			fmt.Println()
			fmt.Println("  Go is not installed on your PATH.")
			fmt.Println("  Install manually with:")
			fmt.Printf("    go install github.com/%s/%s/cmd/waypoint@%s\n", ghOwner, ghRepo, rel.TagName)
			fmt.Println()
			return nil
		}

		module := fmt.Sprintf("github.com/%s/%s/cmd/waypoint@%s", ghOwner, ghRepo, rel.TagName)
		fmt.Printf("  Running: go install %s\n", module)

		c := exec.Command(goPath, "install", module)
		output, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go install failed: %w\n%s", err, string(output))
		}

		fmt.Printf("  Upgraded to %s\n", rel.TagName)

		// Restart the server if it was running before the upgrade.
		if pid > 0 {
			fmt.Printf("  Restarting server...\n")
			restartPort := startFlags.port
			daemonArgs := []string{
				os.Args[0], "start",
				"--port", strconv.Itoa(restartPort),
				"--no-open",
				"--daemon",
			}
			rc := exec.Command(daemonArgs[0], daemonArgs[1:]...)
			rc.Stdin = nil
			rc.Stdout = nil
			rc.Stderr = nil
			detachProcess(rc)
			if err := rc.Start(); err != nil {
				fmt.Printf("  Warning: could not restart server: %v\n", err)
				fmt.Printf("  Run 'waypoint start' manually.\n")
			} else {
				_ = os.WriteFile(pidFilePath(), []byte(fmt.Sprintf("%d", rc.Process.Pid)), 0644)
				fmt.Printf("  Server restarted in background (PID: %d)\n", rc.Process.Pid)
				fmt.Printf("  http://127.0.0.1:%d\n", restartPort)
			}
		} else {
			fmt.Printf("  Run 'waypoint start' to launch the server.\n")
		}

		// Offer skill upgrades for installed skills that differ from embedded version.
		noSkills, _ := cmd.Flags().GetBool("no-skills")
		if !noSkills {
			offerSkillUpgrade()
		}

		fmt.Println()
		return nil
	},
}

// semverCompare returns -1 if a < b, 0 if a == b, 1 if a > b.
// Supports semver 2.0 pre-release suffixes: 1.0.0-alpha < 1.0.0-beta < 1.0.0-rc1 < 1.0.0.
// Versions without a pre-release suffix have higher precedence than those with one.
func semverCompare(a, b string) int {
	aVer, aPre := parseSemver(a)
	bVer, bPre := parseSemver(b)

	// Compare numeric version parts (major.minor.patch)
	min := len(aVer)
	if len(bVer) < min {
		min = len(bVer)
	}
	for i := 0; i < min; i++ {
		if aVer[i] < bVer[i] {
			return -1
		}
		if aVer[i] > bVer[i] {
			return 1
		}
	}
	// Longer version wins (1.0.0 > 1.0)
	switch {
	case len(aVer) < len(bVer):
		return -1
	case len(aVer) > len(bVer):
		return 1
	}

	// Numeric versions are equal — compare pre-release.
	// No pre-release > has pre-release (1.0.0 > 1.0.0-rc1)
	if aPre == "" && bPre != "" {
		return 1
	}
	if aPre != "" && bPre == "" {
		return -1
	}
	// Both have pre-release — compare lexically (alpha < beta < rc1)
	switch {
	case aPre < bPre:
		return -1
	case aPre > bPre:
		return 1
	}
	return 0
}

// parseSemver splits a version string into numeric parts and a pre-release suffix.
// e.g. "1.0.0-rc1" → ([1, 0, 0], "rc1")
func parseSemver(v string) (nums []int, preRelease string) {
	// Strip leading 'v' if present
	v = strings.TrimPrefix(v, "v")

	// Split version and pre-release
	if idx := strings.Index(v, "-"); idx != -1 {
		preRelease = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	nums = make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		nums = append(nums, n)
	}
	return nums, preRelease
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


