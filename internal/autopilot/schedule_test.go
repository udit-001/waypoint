package autopilot

import (
	"testing"
	"time"
)

// CycleDue decides when the daemon fires a cycle. Table-tested here;
// startAutopilotTicker (cli adapter) just polls it.
func TestCycleDue(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	rfc := func(t time.Time) string { return t.Format(time.RFC3339) }
	hour := func(n int) time.Time { return now.Add(time.Duration(-n) * time.Hour) }

	tests := []struct {
		name         string
		enabled      bool
		cadence      time.Duration
		lastFinished string // RFC3339, "" = never ran
		requested    bool   // run-request marker present
		want         bool
		wantReason   string
	}{
		{
			name:    "disabled never runs",
			enabled: false,
			cadence: 6 * time.Hour,
			want:    false,
		},
		{
			name:       "enabled and never ran → first run",
			enabled:    true,
			cadence:    6 * time.Hour,
			want:       true,
			wantReason: "first run",
		},
		{
			name:         "cadence elapsed since last finish",
			enabled:      true,
			cadence:      6 * time.Hour,
			lastFinished: rfc(hour(7)),
			want:         true,
			wantReason:   "cadence due",
		},
		{
			name:         "within cadence waits",
			enabled:      true,
			cadence:      6 * time.Hour,
			lastFinished: rfc(hour(1)),
			want:         false,
		},
		{
			name:         "run request overrides fresh last run",
			enabled:      true,
			cadence:      6 * time.Hour,
			lastFinished: rfc(hour(1)),
			requested:    true,
			want:         true,
			wantReason:   "requested",
		},
		{
			name:       "request with no last run is first run",
			enabled:    true,
			cadence:    6 * time.Hour,
			requested:  true,
			want:       true,
			wantReason: "first run",
		},
		{
			name:         "zero cadence defaults to 6h",
			enabled:      true,
			cadence:      0,
			lastFinished: rfc(hour(3)),
			want:         false, // within default 6h
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := CycleDue(tt.enabled, tt.cadence, tt.lastFinished, tt.requested, now)
			if got != tt.want {
				t.Errorf("CycleDue = %v (%s), want %v", got, reason, tt.want)
			}
			if tt.want && reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
