package cli

import (
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills for this project",
	Long: `Install the job-tracker skill into your AI coding agent so it
knows how to use the CLI to manage job applications.

Supports: opencode, claude-code, codex, pi.dev`,
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the job-tracker skill into an AI agent",
	Long: `Interactively install the job-tracker skill for your AI coding agent.
The skill teaches the agent how to use the job-tracker CLI commands.

Supported agents:
  opencode     Installs to .opencode/skills/job-tracker/SKILL.md
  claude-code  Installs to .claude/skills/job-tracker/SKILL.md
  codex        Installs to .codex/skills/job-tracker/SKILL.md
  pi.dev       Installs to .pi/skills/job-tracker/SKILL.md

Run without flags for interactive mode, or pass --agent to skip prompts.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillsInstall(cmd)
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsInstallCmd.Flags().String("agent", "", "Agent to install for (opencode, claude-code, codex, pi)")
}
