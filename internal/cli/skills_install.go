package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jobtracker/internal/skills"
	"github.com/spf13/cobra"
)

type agentTarget struct {
	name string
	path string
}

var agents = []agentTarget{
	{name: "opencode", path: ".opencode/skills/job-tracker/SKILL.md"},
	{name: "claude-code", path: ".claude/skills/job-tracker/SKILL.md"},
	{name: "codex", path: ".codex/skills/job-tracker/SKILL.md"},
	{name: "pi.dev", path: ".pi/skills/job-tracker/SKILL.md"},
}

func runSkillsInstall(cmd *cobra.Command) error {
	agent, _ := cmd.Flags().GetString("agent")

	var selected agentTarget
	if agent != "" {
		for _, a := range agents {
			if a.name == agent {
				selected = a
				break
			}
		}
		if selected.name == "" {
			return fmt.Errorf("unknown agent %q\n  Supported: opencode, claude-code, codex, pi.dev", agent)
		}
	} else {
		selected = pickAgent()
	}

	targetPath := selected.path

	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil {
		fmt.Printf("  %s already exists.\n", targetPath)
		fmt.Print("  Overwrite? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("  Skipped.")
			return nil
		}
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	// Write the skill file
	if err := os.WriteFile(targetPath, skills.SkillFile, 0644); err != nil {
		return fmt.Errorf("write skill: %w", err)
	}

	fmt.Println()
	fmt.Printf("  ✓ Installed job-tracker skill to %s\n", targetPath)
	fmt.Println()
	printNextSteps(selected)
	fmt.Println()
	return nil
}

func pickAgent() agentTarget {
	fmt.Println()
	fmt.Println("  Pick an AI coding agent:")
	fmt.Println()
	for i, a := range agents {
		fmt.Printf("    %d. %s\n", i+1, a.name)
	}
	fmt.Println()
	fmt.Print("  Enter number [1]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return agents[0]
	}

	var n int
	if _, err := fmt.Sscanf(input, "%d", &n); err != nil || n < 1 || n > len(agents) {
		return agents[0]
	}
	return agents[n-1]
}

func printNextSteps(a agentTarget) {
	fmt.Println("  Next steps:")
	fmt.Printf("  - Skills are auto-discovered at session start\n")
	fmt.Printf("  - Ask your agent to manage job applications with job-tracker\n")
}
