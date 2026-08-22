package autopilot

import (
	"time"
)

// PollInterval is how often the daemon's scheduler re-checks whether a
// cycle is due. Cheap (two SQLite reads); small enough that a run
// request from the Settings toggle lands within "seconds".
const PollInterval = 30 * time.Second

// DefaultCadence is the fallback cadence when settings carry zero.
const DefaultCadence = 6 * time.Hour

// CycleDue decides whether the daemon should fire an autopilot cycle
// right now. It is the whole scheduling policy in one pure function:
//
//   - disabled → never
//   - a run was requested (Settings enable / explicit nudge) → now,
//     regardless of cadence — this is what kills the dead air between
//     enabling and first results
//   - never ran → first run, immediately
//   - otherwise → once the last FINISHED run is a full cadence old
//     (a cycle that overruns its cadence doesn't double-fire)
//
// lastFinishedAt is RFC3339 ("" = never). now is injected for tests.
func CycleDue(enabled bool, cadence time.Duration, lastFinishedAt string, requested bool, now time.Time) (bool, string) {
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
	if lastFinishedAt == "" {
		return true, "first run"
	}
	if cadence <= 0 {
		cadence = DefaultCadence
	}
	finished, err := time.Parse(time.RFC3339, lastFinishedAt)
	if err != nil {
		// Unparseable state: treat as never ran — worst case one extra
		// cycle, never a silently stalled loop.
		return true, "first run"
	}
	if now.Sub(finished) >= cadence {
		return true, "cadence due"
	}
	return false, ""
}
