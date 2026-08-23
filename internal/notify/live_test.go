//go:build linux

package notify

// Self-contained live proof of the click round trip: send a real
// notification, then emit the Activated signal exactly as a spec-
// conforming notification daemon would on click, and assert our
// watcher fires the registered action. Env-gated: WAYPOINT_LIVE_NOTIFY=1.

import (
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestLiveClickActionRoundTrip(t *testing.T) {
	if os.Getenv("WAYPOINT_LIVE_NOTIFY") != "1" {
		t.Skip("live smoke test — set WAYPOINT_LIVE_NOTIFY=1 to run")
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		t.Fatalf("session bus: %v", err)
	}

	clicked := make(chan struct{}, 1)
	if err := sendDBus("Waypoint · action probe", "Automated click-path check", func() {
		clicked <- struct{}{}
	}); err != nil {
		t.Fatalf("sendDBus: %v", err)
	}
	time.Sleep(300 * time.Millisecond) // let the watcher subscribe

	id := lastID.Load()
	if id == 0 {
		t.Fatal("no notification id recorded")
	}
	if err := conn.Emit("/org/freedesktop/Notifications",
		dbusIface+".ActionInvoked", id, "default"); err != nil {
		t.Fatalf("emit ActionInvoked: %v", err)
	}

	select {
	case <-clicked:
		// Full loop proven: Notify accepted → Activated matched → action ran.
	case <-time.After(10 * time.Second):
		t.Fatal("Activated signal never fired the registered action")
	}
}
