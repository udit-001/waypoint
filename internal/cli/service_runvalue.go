package cli

import (
	"fmt"
	"strings"
)

// The Windows logon entry is a value under the per-user Run key. It is written
// with the registry API, so it needs no admin, no COM, no PowerShell and no
// reg.exe — and reading it back is a plain registry read, which is what lets
// `waypoint service status` report the entry honestly.
//
// Two facts shape the value: Windows documents a 260-character limit for Run
// commands, and `service run` must be the entry point rather than `service
// supervise`. A process Explorer creates at logon owns a console window for
// as long as it lives; `service run` hands off to a detached supervisor and
// exits at once, so the window lasts milliseconds instead of forever.
const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "Waypoint"
	// maxRunCommand is the documented Run/RunOnce limit. Over it, Windows
	// silently does not run the entry — so we refuse up front instead.
	maxRunCommand = 260
)

// runValue renders the logon command for exe. It refuses paths Windows would
// refuse or that the 260-character limit would silently drop: a rejected
// install is a fixable error, a silently ignored one is not.
//
// The path is wrapped in plain double quotes, not Go-escaped: a Windows
// command line has no backslash escaping, and a Windows path cannot contain a
// quote — which is exactly why a quote in the path is a refusal.
func runValue(exe string) (string, error) {
	if strings.ContainsRune(exe, '"') {
		return "", fmt.Errorf("the path %s contains a quote, which cannot be quoted exactly", exe)
	}
	command := `"` + exe + `" ` + strings.Join(supervisorEntryArgv(), " ")
	if len(command) > maxRunCommand {
		return "", fmt.Errorf("the startup command would be %d characters, and Windows runs at most %d from the Run key. Move waypoint.exe or its config directory to a shorter path", len(command), maxRunCommand)
	}
	return command, nil
}

// parseRunValue recovers the binary path from a value written by runValue.
// Anything else — a hand-edited or foreign value — reports false, so `service
// restart` can refuse rather than run something we did not write.
func parseRunValue(command string) (string, bool) {
	rest, ok := strings.CutPrefix(command, `"`)
	if !ok {
		return "", false
	}
	exe, tail, ok := strings.Cut(rest, `" `)
	if !ok || exe == "" {
		return "", false
	}
	if tail != strings.Join(supervisorEntryArgv(), " ") {
		return "", false
	}
	return exe, true
}

// supervisorEntryArgv is the logon entry point: start the supervisor, exit.
func supervisorEntryArgv() []string { return []string{"service", "run"} }

// supervisorArgv is the process that actually supervises the server.
func supervisorArgv(exe string) []string {
	return []string{exe, "service", "supervise"}
}
