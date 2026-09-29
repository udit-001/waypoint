//go:build windows

package cli

// probeIdentity is the Windows adapter. There is no portable identity or
// liveness probe — os.FindProcess succeeds for any pid — so Known stays
// false and decideStop falls back to port health. That fallback is what
// keeps a reused PID safe: Windows kills with `taskkill /T /F`, a
// force-kill of the whole process tree, and a PID file with no port to
// vouch for the process is therefore never signaled.
func probeIdentity(info *pidInfo) identityProbe {
	if info == nil || info.PID <= 0 {
		return identityProbe{}
	}
	return identityProbe{
		Alive:       processAlive(info.PID),
		Known:       false,
		PortHealthy: info.Port > 0 && isServerRunning(info.Port),
	}
}

// pidIsOurs on Windows can only answer "is it alive": there is no portable
// executable probe, and refusing to take over a live pid is the safer error —
// the cost is a supervisor that must be stopped by hand, the alternative is a
// second supervisor fighting the first for the port.
func pidIsOurs(pid int, recordedExe string) bool {
	return processAlive(pid)
}
