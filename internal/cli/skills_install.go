package cli

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/SwatiBio/waypoint/internal/skills"
	"github.com/spf13/cobra"
)

type agentTarget struct {
	name string
	dir  string
}

var agents = []agentTarget{
	{name: "opencode", dir: ".opencode/skills/waypoint"},
	{name: "claude-code", dir: ".claude/skills/waypoint"},
	{name: "codex", dir: ".codex/skills/waypoint"},
	{name: "pi.dev", dir: ".pi/skills/waypoint"},
}

func runSkillsInstall(cmd *cobra.Command, args []string) error {
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

	// Confirm overwrite if the target dir already exists.
	if _, err := os.Stat(selected.dir); err == nil {
		fmt.Printf("  %s/ already exists.\n", selected.dir)
		fmt.Print("  Overwrite? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("  Skipped.")
			return nil
		}
	}

	n, err := installSkillDir(skills.Files, skills.SkillName, selected.dir)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  ✓ Installed waypoint skill to %s (%d files)\n", selected.dir, n)
	fmt.Println()
	printNextSteps(selected)
	fmt.Println()
	return nil
}

// installSkillDir walks srcDir inside fsys and writes every file under destDir,
// preserving relative structure. Returns the number of files written.
func installSkillDir(fsys embed.FS, srcDir, destDir string) (int, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, fmt.Errorf("create directories: %w", err)
	}

	count := 0
	err := fs.WalkDir(fsys, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		data, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		out := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
		count++
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("install skill: %w", err)
	}
	return count, nil
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
	fmt.Printf("  - Ask your agent to manage job applications with waypoint\n")
}
