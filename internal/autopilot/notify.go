package autopilot

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// ShortlistNotice is the payload of the one nudge per cycle: how many
// postings were shortlisted this cycle and the strongest among them.
type ShortlistNotice struct {
	Count      int    // total shortlisted this cycle
	TopTitle   string // strongest match this cycle, by score
	TopCompany string
	TopScore   int // 0 when no score was recorded
}

// Notifier delivers the per-cycle nudge. It is the seam that keeps the
// cycle free of delivery mechanics — desktop notifications today, Web
// Push or anything else behind the same one method later. A nil
// Notifier is a valid noop; failures never block scoring.
type Notifier interface {
	NotifyShortlist(ctx context.Context, n ShortlistNotice) error
}

// buildNotice reads the ledger for shortlisted postings from THIS
// cycle (cycleURLs) and picks the strongest by recorded score
// (ties/unscored fall back to first-seen order). Count is per-cycle,
// so the top pick must be too — otherwise "N new matches" could name
// a posting shortlisted weeks ago.
func buildNotice(ctx context.Context, store interface {
	ListPostings(status string) ([]db.Posting, error)
}, count int, cycleURLs map[string]bool) ShortlistNotice {
	n := ShortlistNotice{Count: count}
	postings, err := store.ListPostings(db.StatusShortlisted)
	if err != nil {
		log.Printf("autopilot: notify: read shortlisted: %v", err)
		return n
	}
	for _, p := range postings {
		if !cycleURLs[p.Result.URL] {
			continue
		}
		score, _ := strconv.Atoi(p.Result.Metadata["score"])
		if score > n.TopScore || n.TopTitle == "" {
			n.TopScore = score
			n.TopTitle = p.Result.Title
			n.TopCompany = p.Result.Company
		}
	}
	return n
}

// notifyShortlist sends the nudge through cfg.Notifier when shortlists
// exist this cycle. Errors are logged into the run row — the loop's
// "on never lies" surface — but never fail the cycle.
func notifyShortlist(ctx context.Context, cfg CycleConfig, logEntry *db.RunLog, shortlisted int, allPostings []scraper.Result) {
	if cfg.Notifier == nil || shortlisted <= 0 {
		return
	}
	cycleURLs := make(map[string]bool, len(allPostings))
	for _, p := range allPostings {
		cycleURLs[p.URL] = true
	}
	if err := cfg.Notifier.NotifyShortlist(ctx, buildNotice(ctx, cfg.Store, shortlisted, cycleURLs)); err != nil {
		log.Printf("autopilot: notify failed: %v", err)
		logEntry.Errors = addError(logEntry.Errors, fmt.Sprintf("notify: %v", err))
	}
}
