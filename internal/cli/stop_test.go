package cli

import (
	"strings"
	"testing"
)

func TestDecideStop(t *testing.T) {
	tests := []struct {
		name string
		info *pidInfo
		p    identityProbe
		want stopOutcome
	}{
		{
			name: "no pid file",
			info: nil,
			want: stopNoServer,
		},
		{
			name: "zero pid",
			info: &pidInfo{Port: 8080, PID: 0},
			want: stopNoServer,
		},
		// Executable probe available (unix): identity is authoritative.
		{
			name: "known, dead pid",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{Known: true},
			want: stopStalePID,
		},
		{
			name: "known, alive, not waypoint",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{Known: true, Alive: true},
			want: stopForeignPID,
		},
		{
			name: "known, ours",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{Known: true, Alive: true, Ours: true},
			want: stopStopped,
		},
		{
			name: "known, ours, port dead (shutdown race)",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{Known: true, Alive: true, Ours: true},
			want: stopStopped,
		},
		{
			name: "identity wins over port health for a reused pid",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{Known: true, Alive: true, Ours: false, PortHealthy: true},
			want: stopForeignPID,
		},
		// No executable probe (windows): port health is the only evidence.
		{
			name: "unknown, port healthy",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{PortHealthy: true},
			want: stopStopped,
		},
		{
			name: "unknown, alive, port silent",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{Alive: true},
			want: stopForeignPID,
		},
		{
			name: "unknown, dead",
			info: &pidInfo{Port: 8080, PID: 42},
			p:    identityProbe{},
			want: stopStalePID,
		},
		{
			name: "unknown, no port recorded — refuse rather than signal blind",
			info: &pidInfo{Port: 0, PID: 42},
			p:    identityProbe{Alive: true},
			want: stopUnverifiable,
		},
		{
			name: "unknown, no port recorded, process gone",
			info: &pidInfo{Port: 0, PID: 42},
			p:    identityProbe{},
			want: stopUnverifiable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideStop(tc.info, tc.p); got != tc.want {
				t.Errorf("decideStop(%+v, %+v) = %d, want %d", tc.info, tc.p, got, tc.want)
			}
		})
	}
}

func TestStopMessageNamesThePID(t *testing.T) {
	if got := stopMessage(stopStopped, &pidInfo{PID: 42}); !strings.Contains(got, "42") {
		t.Errorf("stopMessage(stopStopped) = %q, want the pid named", got)
	}
	if got := stopMessage(stopUnverifiable, &pidInfo{PID: 42}); !strings.Contains(got, "not stopped") {
		t.Errorf("stopMessage(stopUnverifiable) = %q, want an explicit refusal", got)
	}
}
