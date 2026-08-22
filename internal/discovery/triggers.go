package discovery

import "time"

// Discovery trigger reasons — why an autopilot cycle decided to run
// discovery before hunting. Empty means don't run.
const (
	TriggerFirstRun    = "first-run"
	TriggerBriefChange = "brief-changed"
	TriggerIntervalDue = "interval-due"
)

// TriggerInput is everything the decision needs. Clock and state are
// injected so the matrix is unit-testable without a database.
type TriggerInput struct {
	Now           time.Time
	IntervalDays  int    // new setting, default 30; <=0 disables the interval arm
	Candidates    int    // candidates ever discovered (any status)
	BriefHash     string // digest of the current brief text
	HasLastRun    bool
	LastBriefHash string
	LastRunAt     time.Time
}

// ShouldRunDiscovery decides whether the autopilot runs discovery before
// this cycle. Priority: first-run > brief-changed > interval-due.
func ShouldRunDiscovery(in TriggerInput) string {
	if in.Candidates == 0 {
		return TriggerFirstRun
	}
	if in.HasLastRun && in.BriefHash != "" && in.LastBriefHash != in.BriefHash {
		return TriggerBriefChange
	}
	if !in.HasLastRun {
		return "" // candidates exist but no recorded run: nothing to compare
	}
	interval := in.IntervalDays
	if interval <= 0 {
		return ""
	}
	if in.Now.Sub(in.LastRunAt) >= time.Duration(interval)*24*time.Hour {
		return TriggerIntervalDue
	}
	return ""
}
