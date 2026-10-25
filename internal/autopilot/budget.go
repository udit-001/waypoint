package autopilot

import (
	"sort"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// DefaultScoredCap and DefaultWall are provisional pending live
// measurement of real cycles (WP-190 lanes).
const (
	DefaultScoredCap = 50
	DefaultWall      = 15 * time.Minute
)

// Budget is one cycle's resource envelope: how long the wall clock may
// run and how many postings may escalate to zen. It is an immutable
// value — no locks, no goroutines. Zero fields keep their meaning, so a
// partially built Budget sets only the limits it cares about.
type Budget struct {
	Deadline  time.Time // zero = no wall limit
	MaxScored int       // 0 = no cap
}

// Expired reports whether the wall limit is spent. A budget without a
// deadline never expires.
func (b Budget) Expired() bool {
	return !b.Deadline.IsZero() && time.Now().After(b.Deadline)
}

// ApplyCap keeps the newest MaxScored postings, date-sorted newest
// first — the same comparator as the degraded-path unscored cap. Under
// the cap (or without one) the slice passes through untouched.
func (b Budget) ApplyCap(ps []scraper.Result) []scraper.Result {
	if b.MaxScored <= 0 || len(ps) <= b.MaxScored {
		return ps
	}
	sort.SliceStable(ps, func(i, k int) bool { return ps[i].Date > ps[k].Date })
	return ps[:b.MaxScored]
}
