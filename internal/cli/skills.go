package cli

import (
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills for this project",
	Long: `Install the waypoint skill into your AI coding agent so it
knows how to use the CLI to manage job applications.

Supports: opencode, claude-code, codex, pi.dev`,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the waypoint skill into an AI agent",
	Long: `Interactively install the waypoint skill for your AI coding agent.
The skill teaches the agent how to use the waypoint CLI commands.

Supported agents:
  opencode     Installs to .opencode/skills/waypoint/
  claude-code  Installs to .claude/skills/waypoint/
  codex        Installs to .codex/skills/waypoint/
  pi.dev       Installs to .pi/skills/waypoint/

Installs the full skill (SKILL.md + references/).

Run without flags for interactive mode, or pass --agent to skip prompts.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillsInstall(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsCheckCmd)
	skillsInstallCmd.Flags().String("agent", "", "Agent to install for (opencode, claude-code, codex, pi)")
}
