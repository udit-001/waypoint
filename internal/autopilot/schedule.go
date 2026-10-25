package autopilot

import (
	"time"
)

// PollInterval is how often the daemon's scheduler re-checks whether a
// cycle is due. Cheap (two SQLite reads); small enough that a run
// request from the Settings toggle lands within "seconds".
const PollInterval = 30 * time.Second

// DefaultWorkingCadence and DefaultDrainInterval are the in-window
// firing rates, provisional pending live measurement (WP-196): the
// steady-state interval, and the tighter interval while the "new"
// backlog still fits no single cycle.
const (
	DefaultWorkingCadence = 45 * time.Minute
	DefaultDrainInterval  = 30 * time.Minute
)

// WorkWindow is the part of the day the user works, in minutes from
// midnight (local time).
type WorkWindow struct{ Start, End int }

// DefaultWorkWindow is 09:00–18:00.
var DefaultWorkWindow = WorkWindow{Start: 540, End: 1080}

// Contains reports whether t falls in the window. A window whose end is
// not after its start is misconfigured; it contains everything — never
// gating is the safer failure than never firing.
func (w WorkWindow) Contains(t time.Time) bool {
	if w.End <= w.Start {
		return true
	}
	m := t.Hour()*60 + t.Minute()
	return m >= w.Start && m < w.End
}

// Schedule is the autopilot's firing policy as one value: where the
// working day sits, how fast to cycle inside it, and how much "new"
// backlog remains (0 = drained).
type Schedule struct {
	Work           WorkWindow
	WorkingCadence time.Duration // in-window steady-state interval
	DrainInterval  time.Duration // in-window interval while Backlog > 0
	Backlog        int           // ledger "new" depth; recomputed each poll
}

// CycleDue decides whether the daemon should fire an autopilot cycle
// right now. It is the whole scheduling policy in one pure function:
//
//   - disabled → never
//   - a run was requested (Settings enable / explicit nudge) → now,
//     regardless of the window — user action outranks the schedule
//   - outside the work window → never (no overnight scheduling: the
//     machine sleeps, and the elapsed check fires on the first
//     in-window poll after waking)
//   - never ran → first run, immediately
//   - otherwise → once the last FINISHED run is a full interval old
//     (DrainInterval while backlog remains, else WorkingCadence; a
//     cycle that overruns its interval doesn't double-fire)
//
// lastFinishedAt is RFC3339 ("" = never). now is injected for tests.
func CycleDue(enabled bool, s Schedule, lastFinishedAt string, requested bool, now time.Time) (bool, string) {
	if !enabled {
		return false, ""
	}
	if requested {
		// A first-run request reports as first run below only when there
		// is genuinely no prior run; an explicit request on top of a
		// fresh run still wins — someone just asked for work.
		if lastFinishedAt == "" {
			return true, "first run"
		}
		return true, "requested"
	}
	if !s.Work.Contains(now) {
		return false, "outside work window"
	}
	if lastFinishedAt == "" {
		return true, "first run"
	}
	interval := s.WorkingCadence
	if interval <= 0 {
		interval = DefaultWorkingCadence
	}
	if s.Backlog > 0 {
		interval = s.DrainInterval
		if interval <= 0 {
			interval = DefaultDrainInterval
		}
	}
	finished, err := time.Parse(time.RFC3339, lastFinishedAt)
	if err != nil {
		// Unparseable state: treat as never ran — worst case one extra
		// cycle, never a silently stalled loop.
		return true, "first run"
	}
	if now.Sub(finished) >= interval {
		return true, "cadence due"
	}
	return false, ""
}
