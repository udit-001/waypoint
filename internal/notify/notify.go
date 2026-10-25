// Package notify delivers the daemon's desktop notifications: shortlists
// announcing themselves as they land during an autopilot cycle (WP-195),
// coalesced per quiet window by the caller. It satisfies the
// autopilot.Notifier seam so delivery mechanics stay out of the cycle —
// Web Push or any future channel slots in behind the same interface.
//
// Platform reality, v1:
//   - Linux: freedesktop D-Bus with a "default" action; clicking runs
//     xdg-open on the Found Jobs page.
//   - macOS (osascript) and Windows (PowerShell toast): delivery only —
//     neither shell path can open a URL on click.
//
// The queue is always the source of truth; a notification is a nudge,
// never a carrier of state.
package notify

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/udit-001/waypoint/internal/autopilot"
)

const appName = "Waypoint"

// Desktop sends system notifications for cycle results.
type Desktop struct {
	// OpenURL is the page a click should open where the OS allows
	// (Linux). Empty disables the click action.
	OpenURL string
}

// New returns a Desktop notifier that opens openURL on click (Linux).
func New(openURL string) *Desktop {
	return &Desktop{OpenURL: openURL}
}

// NotifyShortlists implements autopilot.Notifier: one notification per
// coalesced batch of shortlists.
func (d *Desktop) NotifyShortlists(_ context.Context, events []autopilot.ShortlistEvent) error {
	title, body := formatShortlist(events)
	return deliver(title, body, d.clickAction())
}

// NotifyDegraded implements autopilot.Notifier: one notice when a run
// closes degraded (WP-193). An empty error list never delivers — the
// verdict cannot be degraded without errors, so reaching here with none
// means a caller bug, not copy to toast.
func (d *Desktop) NotifyDegraded(_ context.Context, notice autopilot.DegradedNotice) error {
	if len(notice.Errors) == 0 {
		return nil
	}
	title, body := formatDegraded(notice.Errors)
	return deliver(title, body, d.clickAction())
}

func (d *Desktop) clickAction() func() {
	if d.OpenURL == "" {
		return nil
	}
	url := d.OpenURL
	return func() { _ = openInBrowser(url) }
}

// openInBrowser opens url with the platform's default handler.
func openInBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// formatShortlist renders the notification copy: title carries the
// count, body carries the strongest match by score — the first event
// when all are unscored. Pure so tests pin it.
func formatShortlist(events []autopilot.ShortlistEvent) (title, body string) {
	noun := "matches"
	if len(events) == 1 {
		noun = "match"
	}
	title = fmt.Sprintf("%s · %d new %s", appName, len(events), noun)

	top := autopilot.ShortlistEvent{}
	for i, e := range events {
		if i == 0 || e.Score > top.Score {
			top = e
		}
	}
	if top.Title == "" {
		return title, "Review them when you have two minutes."
	}

	topTitle := truncate60(top.Title)
	var b strings.Builder
	b.WriteString("Top pick: ")
	b.WriteString(topTitle)
	if top.Company != "" {
		fmt.Fprintf(&b, " at %s", top.Company)
	}
	if top.Score > 0 {
		fmt.Fprintf(&b, " (%d)", top.Score)
	}
	return title, b.String()
}

// truncate60 clips copy to a toast-sized line, same rule the shortlist
// body uses: 60 runes, ellipsis on the cut.
func truncate60(s string) string {
	if runes := []rune(s); len(runes) > 60 {
		return strings.TrimSpace(string(runes[:57])) + "…"
	}
	return s
}

// formatDegraded renders the degraded-run copy: body carries the first
// stage error (the runlog's own text) plus a count of the rest. Pure so
// tests pin it.
func formatDegraded(errs []string) (title, body string) {
	title = fmt.Sprintf("%s · autopilot run degraded", appName)
	body = truncate60(errs[0])
	if n := len(errs) - 1; n > 0 {
		body += fmt.Sprintf(" +%d more", n)
	}
	return title, body
}
