// Package notify delivers the daemon's desktop notifications: one nudge
// per autopilot cycle when shortlists exist. It satisfies the
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

// NotifyShortlist implements autopilot.Notifier.
func (d *Desktop) NotifyShortlist(_ context.Context, n autopilot.ShortlistNotice) error {
	title, body := formatShortlist(n)
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
// count, body carries the strongest match. Pure so tests pin it.
func formatShortlist(n autopilot.ShortlistNotice) (title, body string) {
	noun := "matches"
	if n.Count == 1 {
		noun = "match"
	}
	title = fmt.Sprintf("%s · %d new %s", appName, n.Count, noun)

	if n.TopTitle == "" {
		return title, "Review them when you have two minutes."
	}

	top := n.TopTitle
	if len(top) > 60 {
		top = strings.TrimSpace(top[:57]) + "…"
	}
	var b strings.Builder
	b.WriteString("Top pick: ")
	b.WriteString(top)
	if n.TopCompany != "" {
		fmt.Fprintf(&b, " at %s", n.TopCompany)
	}
	if n.TopScore > 0 {
		fmt.Fprintf(&b, " (%d)", n.TopScore)
	}
	return title, b.String()
}
