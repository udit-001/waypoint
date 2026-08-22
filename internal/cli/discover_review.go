package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
)

// discoverReviewCmd is shared plumbing for `discover add|dismiss`: load
// the candidate by id, apply the review transition, report. The target
// status is passed explicitly — the command's Use string must never
// decide a database write.
func discoverReviewCmd(use, short, long, status string, apply func(cand db.CompanyCandidate) (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("candidate id must be a number, got %q", args[0])
			}

			cands, err := store.Candidates("")
			if err != nil {
				return formatError("list candidates", err)
			}
			var cand *db.CompanyCandidate
			for i := range cands {
				if cands[i].ID == id {
					cand = &cands[i]
					break
				}
			}
			if cand == nil {
				if jsonOut {
					printJSON(map[string]any{"meta": map[string]any{"id": id, "updated": false, "error": "not found"}})
				}
				return fmt.Errorf("no candidate with id %d — run 'waypoint discover run' to list ids", id)
			}
			if cand.Status != db.StatusCandidateSuggested {
				msg := fmt.Sprintf("already %s", cand.Status)
				if jsonOut {
					printJSON(map[string]any{"meta": map[string]any{"id": id, "updated": false, "error": msg}})
				}
				return fmt.Errorf("candidate %q is %s — nothing to do", cand.Name, cand.Status)
			}

			detail, err := apply(*cand)
			if err != nil {
				if jsonOut {
					printJSON(map[string]any{"meta": map[string]any{
						"id": id, "name": cand.Name, "updated": false, "status": status, "error": err.Error(),
					}})
				}
				return formatError(fmt.Sprintf("%s %s", use, cand.Name), err)
			}

			if jsonOut {
				printJSON(map[string]any{"meta": map[string]any{
					"id": id, "name": cand.Name, "updated": true, "status": status, "detail": detail,
				}})
				return nil
			}
			fmt.Printf("  %s — %s (%s)\n", cand.Name, status, detail)
			return nil
		},
	}
}

var discoverAddCmd = discoverReviewCmd(
	"add <id>",
	"Promote a discovered company into boards.toml",
	`Promote one suggested candidate into boards.toml. Ids come from
'waypoint discover run' (candidates[].id in JSON; the ID column in the
table).

Each board runs the same verify gate as 'boards add': page 1 is fetched
before anything is written, so only answering boards land. The entry is
saved enabled — the next 'boards sweep' picks up its postings with no
further steps — and the candidate becomes "added".

No-ops and conflicts:
  already-watched URL   nothing happens; the candidate stays "suggested"
  same-name board       fails — an existing board's URL is never replaced

The JSON meta {id, name, updated, status, detail|error} reports every
outcome on every exit path. Add is done when meta.status is "added".`,
	db.StatusCandidateAdded,
	func(cand db.CompanyCandidate) (string, error) {
		bf, cfg, err := loadBoardsStore()
		if err != nil {
			return "", err
		}

		out, err := discovery.Promote(context.Background(), cand.Name, candidateLinks(cand.Boards), bf)
		if err != nil {
			return "", err
		}
		if out.Added == 0 {
			return "", fmt.Errorf("board(s) already in boards.toml — nothing to add")
		}
		if err := config.SaveBoards(cfg, bf); err != nil {
			return "", err
		}
		if err := store.SetCandidateStatus(cand.ID, db.StatusCandidateAdded); err != nil {
			return "", err
		}

		detail := fmt.Sprintf("%d board(s) added via %s, %d jobs on first page", out.Added, out.Provider, out.Fetched)
		if out.AlreadyWatched > 0 {
			detail += fmt.Sprintf(", %d already watched", out.AlreadyWatched)
		}
		return detail, nil
	},
)

// candidateLinks converts stored board rows into the discovery package's
// link shape for the shared promotion path.
func candidateLinks(bs []db.CandidateBoard) []discovery.BoardLink {
	out := make([]discovery.BoardLink, 0, len(bs))
	for _, b := range bs {
		out = append(out, discovery.BoardLink{Provider: b.Provider, URL: b.URL})
	}
	return out
}

var discoverDismissCmd = discoverReviewCmd(
	"dismiss <id>",
	"Tombstone a discovered company so it is not suggested again",
	`Tombstone one suggested candidate: its status becomes "dismissed",
discovery stops suggesting the company on future runs, and boards.toml
is untouched. Decisions are durable — a dismissed name returns only if
the underlying data is re-seeded.

The JSON meta {id, name, updated, status} reports the outcome. Dismiss
is done when meta.status is "dismissed".`,
	db.StatusCandidateDismissed,
	func(cand db.CompanyCandidate) (string, error) {
		if err := store.SetCandidateStatus(cand.ID, db.StatusCandidateDismissed); err != nil {
			return "", err
		}
		return "will not be suggested again", nil
	},
)

func init() {
	discoverCmd.AddCommand(discoverAddCmd)
	discoverCmd.AddCommand(discoverDismissCmd)
}
