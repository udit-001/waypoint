package cli

import "strconv"

// daemonFlags is the one spelling of "run the daemon in the background", as an
// argument list after the binary name.
//
// Three callers share it, and they have to agree or a flag added for one
// silently does not apply to the others:
//
//   - the service manager (systemd/launchd) runs it as the unit's command;
//   - the supervisor runs it as its child (internal/supervise);
//   - `start --background` runs it as the detached child.
//
// A port of 0 means "take the port from config". That is what the service unit
// and the login entry want: they outlive a run, so a pinned port would go
// stale the moment the config changes. `start --background` passes the port it
// was asked for.
//
// --no-open is redundant under --daemon (the server suppresses the browser when
// Silent), and is kept so the command line reads the same in the service log
// and in `--help` regardless of which caller spawned it.
func daemonFlags(port int) []string {
	flags := []string{"start"}
	if port > 0 {
		flags = append(flags, "--port", strconv.Itoa(port))
	}
	return append(flags, "--no-open", "--daemon")
}
