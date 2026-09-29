package cli

import (
	"strings"
	"testing"
)

func TestRunValueQuotesAPathWithSpaces(t *testing.T) {
	got, err := runValue(`C:\Program Files\Waypoint\waypoint.exe`)
	if err != nil {
		t.Fatalf("runValue: %v", err)
	}
	// The value is the logon entry point: start the supervisor and exit.
	want := `"C:\Program Files\Waypoint\waypoint.exe" service run`
	if got != want {
		t.Fatalf("runValue = %q, want %q", got, want)
	}
	if len(got) > maxRunCommand {
		t.Fatalf("runValue is %d chars, over the %d limit", len(got), maxRunCommand)
	}
}

func TestRunValueRefusesWhatWindowsWouldNotRun(t *testing.T) {
	long := `C:\` + strings.Repeat("d", maxRunCommand) + `\waypoint.exe`
	if _, err := runValue(long); err == nil {
		t.Fatal("a command over the Run key's limit must be refused, not truncated")
	}
	if _, err := runValue(`C:\odd"name\waypoint.exe`); err == nil {
		t.Fatal("a path containing a quote cannot be quoted exactly; must be refused")
	}
}

func TestParseRunValueRoundTrips(t *testing.T) {
	exe := `C:\Program Files\Waypoint\waypoint.exe`
	command, err := runValue(exe)
	if err != nil {
		t.Fatalf("runValue: %v", err)
	}
	got, ok := parseRunValue(command)
	if !ok {
		t.Fatalf("parseRunValue(%q) failed to recognise our own value", command)
	}
	if got != exe {
		t.Fatalf("parseRunValue = %q, want %q", got, exe)
	}
}

func TestParseRunValueRejectsForeignValues(t *testing.T) {
	for _, command := range []string{
		"",
		`"C:\waypoint.exe"`,
		`"C:\waypoint.exe" start --daemon`,
		`C:\waypoint.exe service run`,
		`"C:\waypoint.exe" service run extra`,
	} {
		if exe, ok := parseRunValue(command); ok {
			t.Errorf("parseRunValue(%q) = %q, ok; want rejected", command, exe)
		}
	}
}

// TestRunValueIsWhatWeWouldSpawn guards the coupling between the logon value
// and the command `service start` runs: if one changes, the other must too.
func TestRunValueIsWhatWeWouldSpawn(t *testing.T) {
	exe := "/usr/local/bin/waypoint"
	command, err := runValue(exe)
	if err != nil {
		t.Fatalf("runValue: %v", err)
	}
	spawn := strings.Join(supervisorArgv(exe), " ")
	if !strings.Contains(command, "service run") {
		t.Fatalf("runValue = %q, want the logon entry point", command)
	}
	if !strings.Contains(spawn, "service supervise") {
		t.Fatalf("supervisorArgv = %q, want the supervisor", spawn)
	}
	// Both name the same binary, so a moved install is caught by parseRunValue.
	if !strings.Contains(command, exe) || !strings.Contains(spawn, exe) {
		t.Fatalf("runValue = %q, supervisorArgv = %q; both must name %q", command, spawn, exe)
	}
}
