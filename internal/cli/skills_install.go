package cli

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SwatiBio/waypoint/internal/skills"
	"github.com/spf13/cobra"
)

type agentTarget struct {
	name    string
	dir     string
	detect  func() bool
}

var agents = []agentTarget{
	{name: "opencode", dir: ".opencode/skills/waypoint", detect: func() bool { return hasBinary("opencode") || hasDir(".opencode") }},
	{name: "claude-code", dir: ".claude/skills/waypoint", detect: func() bool { return hasBinary("claude") || hasDir(".claude") }},
	{name: "codex", dir: ".codex/skills/waypoint", detect: func() bool { return hasBinary("codex") || hasDir(".codex") }},
	{name: "pi.dev", dir: ".pi/skills/waypoint", detect: func() bool { return hasBinary("pi") || hasDir(".pi") }},
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
	detected := detectAgents()

	switch len(detected) {
	case 0:
		// None found — show all options
		fmt.Println()
		fmt.Println("  No AI coding agent detected. Pick one:")
		fmt.Println()
		for i, a := range agents {
			fmt.Printf("    %d. %s\n", i+1, a.name)
		}
		fmt.Println()
		fmt.Print("  Enter number [1]: ")
		return readChoice(agents)

	case 1:
		// Exactly one — auto-select
		fmt.Println()
		fmt.Printf("  Detected %s\n", detected[0].name)
		return detected[0]

	default:
		// Multiple — show only detected
		fmt.Println()
		fmt.Println("  Detected AI coding agents:")
		fmt.Println()
		for i, a := range detected {
			fmt.Printf("    %d. %s\n", i+1, a.name)
		}
		fmt.Println()
		fmt.Print("  Enter number [1]: ")
		return readChoice(detected)
	}
}

func detectAgents() []agentTarget {
	var found []agentTarget
	for _, a := range agents {
		if a.detect() {
			found = append(found, a)
		}
	}
	return found
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func hasDir(name string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(home, name)); err == nil {
		return true
	}
	// Also check current directory
	if _, err := os.Stat(name); err == nil {
		return true
	}
	return false
}

func readChoice(list []agentTarget) agentTarget {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return list[0]
	}

	var n int
	if _, err := fmt.Sscanf(input, "%d", &n); err != nil || n < 1 || n > len(list) {
		return list[0]
	}
	return list[n-1]
}

func printNextSteps(a agentTarget) {
	fmt.Println("  Next steps:")
	fmt.Printf("  - Skills are auto-discovered at session start\n")
	fmt.Printf("  - Ask your agent to manage job applications with waypoint\n")
}

// offerSkillInstall is the shared skill-install flow for init.
// It detects agents, branches by count, and installs without
// redundant prompting.
func offerSkillInstall() {
	detected := detectAgents()

	switch len(detected) {
	case 1:
		// One agent detected — install for it without prompting
		fmt.Println()
		fmt.Printf("  Detected %s — installing waypoint skill...\n", detected[0].name)
		n, err := installSkillDir(skills.Files, skills.SkillName, detected[0].dir)
		if err != nil {
			fmt.Printf("  Warning: skill install failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Installed waypoint skill to %s (%d files)\n", detected[0].dir, n)
		}

	case 0:
		// No agent detected — offer to pick from all
		fmt.Println()
		fmt.Print("  No AI coding agent detected. Install the waypoint skill anyway? [y/N] ")
		if promptYes() {
			selected := pickAgent()
			n, err := installSkillDir(skills.Files, skills.SkillName, selected.dir)
			if err != nil {
				fmt.Printf("  Warning: skill install failed: %v\n", err)
			} else {
				fmt.Println()
				fmt.Printf("  ✓ Installed waypoint skill to %s (%d files)\n", selected.dir, n)
			}
		}

	default:
		// Multiple detected — pick which one
		fmt.Println()
		fmt.Print("  Install the waypoint skill for an AI coding agent? [Y/n] ")
		if promptDefaultYes() {
			selected := pickAgent()
			n, err := installSkillDir(skills.Files, skills.SkillName, selected.dir)
			if err != nil {
				fmt.Printf("  Warning: skill install failed: %v\n", err)
			} else {
				fmt.Println()
				fmt.Printf("  ✓ Installed waypoint skill to %s (%d files)\n", selected.dir, n)
				printNextSteps(selected)
			}
		}
	}
}

// offerSkillUpgrade checks for installed waypoint skills that differ from
// the embedded version and offers to update them. Shared with upgrade.
func offerSkillUpgrade() {
	// Find which agents have the skill installed and outdated.
	var outdated []agentTarget
	for _, a := range agents {
		skillFile := filepath.Join(a.dir, "SKILL.md")
		installed, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}
		embedded, err := fs.ReadFile(skills.Files, filepath.Join(skills.SkillName, "SKILL.md"))
		if err != nil {
			continue
		}
		if string(installed) != string(embedded) {
			outdated = append(outdated, a)
		}
	}

	if len(outdated) == 0 {
		return
	}

	fmt.Println()
	if len(outdated) == 1 {
		fmt.Printf("  The waypoint skill for %s has changed. Update it? [Y/n] ", outdated[0].name)
	} else {
		names := make([]string, len(outdated))
		for i, a := range outdated {
			names[i] = a.name
		}
		fmt.Printf("  Waypoint skills have changed for %s. Update them? [Y/n] ", strings.Join(names, ", "))
	}

	if !promptDefaultYes() {
		return
	}

	for _, a := range outdated {
		n, err := installSkillDir(skills.Files, skills.SkillName, a.dir)
		if err != nil {
			fmt.Printf("  Warning: failed to update skill for %s: %v\n", a.name, err)
		} else {
			fmt.Printf("  ✓ Updated %s skill (%d files)\n", a.name, n)
		}
	}
}

// promptYes reads a y/N prompt. Returns true only on explicit "y" or "yes".
func promptYes() bool {
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// promptDefaultYes reads a Y/n prompt. Returns true on enter, "y", or "yes".
func promptDefaultYes() bool {
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "" || answer == "y" || answer == "yes"
}
