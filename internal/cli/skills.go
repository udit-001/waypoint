package cli

import (
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills for this project",
	Long: `Manage the waypoint skill for your AI coding agent.

Run 'waypoint skills check' to see installed copies and their status.`,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the waypoint skill",
	Long: `Install the waypoint skill for your AI coding agent.

Installs to the Agent Skills Open Standard location (~/.agents/skills/)
which is read by opencode, codex, and pi.dev. Also installs to
~/.claude/skills/ for claude-code if detected.

Run without flags for interactive mode.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillsInstall(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsCheckCmd)
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsInstallCmd.Flags().Bool("agents-only", false, "Install only to .agents/skills (opencode, codex, pi.dev)")
	skillsInstallCmd.Flags().Bool("claude-only", false, "Install only to .claude/skills (claude-code)")
	skillsInstallCmd.Flags().Bool("all", false, "Install all detected families")
	skillsInstallCmd.Flags().Bool("project", false, "Install at project level instead of globally")
	skillsUninstallCmd.Flags().Bool("all", false, "Remove all discovered installs")
}
