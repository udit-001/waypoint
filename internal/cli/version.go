package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the waypoint version",
	Long: `Print the waypoint version, commit, and build date.

Same information as 'waypoint --version'/'waypoint -v', as a subcommand —
for agents and scripts that reach for the conventional '<tool> version' form.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		printVersion()
		return nil
	},
}

// printVersion writes the version line(s) the cobra flag also emits, so both
// 'waypoint --version' and 'waypoint version' produce identical output.
func printVersion() {
	fmt.Printf("waypoint version %s\n", version.Version)
	if version.Commit != "" {
		fmt.Printf("  commit: %s\n", version.Commit)
	}
	if version.Date != "" {
		fmt.Printf("  built:  %s\n", version.Date)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
