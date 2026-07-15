package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/udit-001/waypoint/internal/skills"
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

	files, err := skillFilesMap()
	if err != nil {
		return fmt.Errorf("read skill files: %w", err)
	}

	n, err := installSkillDir(files, selected.dir)
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

// installSkillDir writes every file from the map under destDir, then
// writes a manifest for change detection. Returns the number of files written.
func installSkillDir(files map[string][]byte, destDir string) (int, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, fmt.Errorf("create directories: %w", err)
	}

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		out := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return 0, err
		}
		if err := os.WriteFile(out, files[rel], 0o644); err != nil {
			return 0, fmt.Errorf("write %s: %w", out, err)
		}
	}

	if err := skills.WriteManifestFromMap(destDir, files); err != nil {
		return len(files), fmt.Errorf("write manifest: %w", err)
	}

	return len(files), nil
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

	files, err := skillFilesMap()
	if err != nil {
		fmt.Printf("  Warning: could not read skill files: %v\n", err)
		return
	}

	switch len(detected) {
	case 1:
		// One agent detected — install for it without prompting
		fmt.Println()
		fmt.Printf("  Detected %s — installing waypoint skill...\n", detected[0].name)
		n, err := installSkillDir(files, detected[0].dir)
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
			n, err := installSkillDir(files, selected.dir)
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
			n, err := installSkillDir(files, selected.dir)
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

// skillFilesMap walks the embedded skill directory and returns a map of
// relative path → file contents. Used by offerSkillUpgrade to detect
// changes in any skill file, not just SKILL.md.
func skillFilesMap() (map[string][]byte, error) {
	files := make(map[string][]byte)
	prefix := skills.SkillName + "/"
	err := fs.WalkDir(skills.Files, skills.SkillName, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, prefix)
		data, err := skills.Files.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	})
	return files, err
}

// skillFilesChanged returns true if any file in the embedded skill differs
// from the installed copy at dir. Uses the manifest for change detection;
// falls back to byte-by-byte comparison for legacy installs without one.
func skillFilesChanged(dir string, embedded map[string][]byte) bool {
	m, err := skills.ReadManifest(dir)
	if err != nil {
		// No manifest — legacy install, compare file-by-file.
		return legacySkillFilesChanged(dir, embedded)
	}
	if m.Hash != skills.ManifestHash(embedded) {
		return true
	}
	for _, f := range m.Files {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return true
		}
	}
	return false
}

// legacySkillFilesChanged compares every embedded file against the installed
// copy byte-by-byte. Used for skill dirs installed before manifests existed.
func legacySkillFilesChanged(dir string, embedded map[string][]byte) bool {
	for rel, want := range embedded {
		got, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return true
		}
		if string(got) != string(want) {
			return true
		}
	}
	installedCount := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			installedCount++
		}
		return nil
	})
	return installedCount != len(embedded)
}

// offerSkillUpgrade checks for installed waypoint skills that differ from
// the embedded version and offers to update them. Shared with upgrade.
func offerSkillUpgrade() {
	embedded, err := skillFilesMap()
	if err != nil {
		return
	}

	// Find which agents have the skill installed and outdated.
	var outdated []agentTarget
	for _, a := range agents {
		if !isSkillInstalled(a.dir) {
			continue
		}
		if skillFilesChanged(a.dir, embedded) {
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
		n, err := installSkillDir(embedded, a.dir)
		if err != nil {
			fmt.Printf("  Warning: failed to update skill for %s: %v\n", a.name, err)
		} else {
			fmt.Printf("  ✓ Updated %s skill (%d files)\n", a.name, n)
		}
	}
}

// isSkillInstalled returns true if the SKILL.md exists at dir.
func isSkillInstalled(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil
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
