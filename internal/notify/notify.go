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

	topTitle := top.Title
	if runes := []rune(topTitle); len(runes) > 60 {
		topTitle = strings.TrimSpace(string(runes[:57])) + "…"
	}
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
