package cli

import (
	"fmt"
	"os"

	"github.com/udit-001/waypoint/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check installed skills and their status",
	Long: `Report which agent skill directories exist and whether they are
current or outdated.

Reads the manifest (waypoint.skill.json) in each installed skill
directory to determine status.

Examples:
  waypoint skills check`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		embedded, err := skillFilesMap()
		if err != nil {
			return fmt.Errorf("read embedded skill files: %w", err)
		}

		fmt.Println()
		var installed bool
		var outdatedCount int

		for _, a := range agents {
			if !isSkillInstalled(a.dir) {
				continue
			}
			installed = true

			manifestPath := skills.ManifestPath(a.dir)
			_, hasManifest := os.Stat(manifestPath)

			status := "current"
			if skillFilesChanged(a.dir, embedded) {
				status = "outdated"
			}

			icon := "✓"
			if status == "outdated" {
				icon = "⚠"
				outdatedCount++
			}

			unmanaged := ""
			if hasManifest != nil {
				unmanaged = " [unmanaged]"
			}
			fmt.Printf("  %s %s — %s%s\n", icon, a.dir, status, unmanaged)
		}

		if !installed {
			fmt.Println("  No skills installed.")
			fmt.Println("  Run 'waypoint skills install' to install.")
			fmt.Println()
			return nil
		}

		fmt.Println()
		if outdatedCount > 0 {
			fmt.Printf("  %d outdated install(s) found. Run 'waypoint skills install' to update.\n", outdatedCount)
		} else {
			fmt.Println("  All skills are up to date.")
		}
		fmt.Println()
		return nil
	},
}
