package autopilot

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/udit-001/waypoint/internal/db"
)

// ShortlistNotice is the payload of the one nudge per cycle: how many
// postings were shortlisted and the strongest match among them.
type ShortlistNotice struct {
	Count      int    // total shortlisted this cycle
	TopTitle   string // strongest match by score
	TopCompany string
	TopScore   int    // 0 when no score was recorded
	OpenURL    string // Found Jobs page; click target where the OS allows
}

// Notifier delivers the per-cycle nudge. It is the seam that keeps the
// cycle free of delivery mechanics — desktop notifications today, Web
// Push or anything else behind the same one method later. A nil
// Notifier is a valid noop; failures never block scoring.
type Notifier interface {
	NotifyShortlist(ctx context.Context, n ShortlistNotice) error
}

// buildNotice reads the ledger for shortlisted postings and picks the
// strongest by recorded score (ties/unscored fall back to first-seen).
func buildNotice(ctx context.Context, store interface {
	ListPostings(status string) ([]db.Posting, error)
}, count int) ShortlistNotice {
	n := ShortlistNotice{Count: count}
	postings, err := store.ListPostings(db.StatusShortlisted)
	if err != nil {
		log.Printf("autopilot: notify: read shortlisted: %v", err)
		return n
	}
	for _, p := range postings {
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
func notifyShortlist(ctx context.Context, cfg CycleConfig, logEntry *db.RunLog, shortlisted int) {
	if cfg.Notifier == nil || shortlisted <= 0 {
		return
	}
	if err := cfg.Notifier.NotifyShortlist(ctx, buildNotice(ctx, cfg.Store, shortlisted)); err != nil {
		log.Printf("autopilot: notify failed: %v", err)
		logEntry.Errors = addError(logEntry.Errors, fmt.Sprintf("notify: %v", err))
	}
}
