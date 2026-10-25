package autopilot

import (
	"testing"
	"time"
)

// CycleDue decides when the daemon fires a cycle. Table-tested here;
// startAutopilotTicker (cli adapter) just polls it.
func TestCycleDue(t *testing.T) {
	// 2026-08-22 is a Saturday; 12:00 UTC sits inside the default window.
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	rfc := func(t time.Time) string { return t.Format(time.RFC3339) }
	min := func(n int) time.Time { return now.Add(time.Duration(-n) * time.Minute) }

	// Full-day window: every wall-clock minute is in-window, so window
	// effects stay out of the interval cases below.
	always := WorkWindow{Start: 0, End: 1440}
	night := WorkWindow{Start: 0, End: 60} // 00:00–01:00; 12:00 is outside
	working := 45 * time.Minute
	drain := 30 * time.Minute

	tests := []struct {
		name         string
		enabled      bool
		schedule     Schedule
		lastFinished string // RFC3339, "" = never ran
		requested    bool   // run-request marker present
		want         bool
		wantReason   string
	}{
		{
			name:    "disabled never runs",
			enabled: false,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			want: false,
		},
		{
			name:    "in-window and drained fires on working cadence",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			lastFinished: rfc(min(45)),
			want:         true,
			wantReason:   "cadence due",
		},
		{
			name:    "in-window and drained waits under the working cadence",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			lastFinished: rfc(min(44)),
			want:         false,
		},
		{
			name:    "backlog drains on the drain interval, not the working cadence",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
				Backlog:        3,
			},
			lastFinished: rfc(min(35)), // past drain (30m), under working (45m)
			want:         true,
			wantReason:   "cadence due",
		},
		{
			name:    "backlog waits under the drain interval",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
				Backlog:        3,
			},
			lastFinished: rfc(min(29)),
			want:         false,
		},
		{
			name:    "outside the window never fires",
			enabled: true,
			schedule: Schedule{
				Work:           night,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			lastFinished: rfc(min(120)),
			want:         false,
			wantReason:   "outside work window",
		},
		{
			name:    "run request fires outside the window",
			enabled: true,
			schedule: Schedule{
				Work:           night,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			lastFinished: rfc(min(10)),
			requested:    true,
			want:         true,
			wantReason:   "requested",
		},
		{
			name:    "request with no last run is first run",
			enabled: true,
			schedule: Schedule{
				Work:           night,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			requested:  true,
			want:       true,
			wantReason: "first run",
		},
		{
			name:    "in-window and never ran → first run",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			want:       true,
			wantReason: "first run",
		},
		{
			name:    "outside the window the first run waits too",
			enabled: true,
			schedule: Schedule{
				Work:           night,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			want:       false,
			wantReason: "outside work window",
		},
		{
			name:    "wake catch-up: elapsed far past the cadence fires in-window",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			lastFinished: rfc(now.Add(-8 * time.Hour)), // slept through the interval
			want:         true,
			wantReason:   "cadence due",
		},
		{
			name:    "zero working cadence defaults to 45m",
			enabled: true,
			schedule: Schedule{
				Work:          always,
				DrainInterval: drain,
			},
			lastFinished: rfc(min(44)),
			want:         false, // within the default 45m
		},
		{
			name:    "zero drain interval defaults to 30m",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				Backlog:        2,
			},
			lastFinished: rfc(min(30)),
			want:         true,
			wantReason:   "cadence due",
		},
		{
			name:    "unparseable last finish is treated as never ran",
			enabled: true,
			schedule: Schedule{
				Work:           always,
				WorkingCadence: working,
				DrainInterval:  drain,
			},
			lastFinished: "not-a-time",
			want:         true,
			wantReason:   "first run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := CycleDue(tt.enabled, tt.schedule, tt.lastFinished, tt.requested, now)
			if got != tt.want {
				t.Errorf("CycleDue = %v (%s), want %v", got, reason, tt.want)
			}
			if tt.want && reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

// A misconfigured window (end not after start) contains everything —
// never gating is the safer failure than never firing.
func TestWorkWindowContainsInverted(t *testing.T) {
	inverted := WorkWindow{Start: 1080, End: 540}
	if !inverted.Contains(time.Date(2026, 8, 22, 3, 0, 0, 0, time.UTC)) {
		t.Error("inverted window excluded 03:00; want always-contained")
	}
	if !inverted.Contains(time.Date(2026, 8, 22, 23, 59, 0, 0, time.UTC)) {
		t.Error("inverted window excluded 23:59; want always-contained")
	}
}

func TestWorkWindowContains(t *testing.T) {
	w := WorkWindow{Start: 540, End: 1080} // 09:00–18:00
	cases := []struct {
		hour, minute int
		want         bool
	}{
		{9, 0, true},
		{17, 59, true},
		{18, 0, false},
		{8, 59, false},
		{0, 0, false},
	}
	for _, c := range cases {
		at := time.Date(2026, 8, 22, c.hour, c.minute, 0, 0, time.UTC)
		if got := w.Contains(at); got != c.want {
			t.Errorf("Contains(%02d:%02d) = %v, want %v", c.hour, c.minute, got, c.want)
		}
	}
}
