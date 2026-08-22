//go:build linux

package notify

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
)

const (
	dbusName  = "org.freedesktop.Notifications"
	dbusPath  = "/org/freedesktop/Notifications"
	dbusIface = "org.freedesktop.Notifications"
)

// watcher state: the Activated signal carries only the notification id,
// so the most recent notice's click action is what a click resolves to.
var (
	watchOnce sync.Once
	lastID    atomic.Uint32
	lastClick atomic.Pointer[func()]
)

// sendDBus is the test seam around the D-Bus round trip.
var sendDBus = func(title, body string, onClick func()) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}

	actions := []string{}
	if onClick != nil {
		// "default" is the ordinary-click action in the freedesktop spec.
		actions = []string{"default", "Open Waypoint"}
	}

	obj := conn.Object(dbusName, dbusPath)
	call := obj.Call(dbusIface+".Notify", 0,
		appName, uint32(0), "", title, body, actions,
		map[string]dbus.Variant{}, int32(-1))
	if call.Err != nil {
		return fmt.Errorf("notify call: %w", call.Err)
	}
	if len(call.Body) > 0 {
		if id, ok := call.Body[0].(uint32); ok {
			lastID.Store(id)
			lastClick.Store(&onClick)
		}
	}

	watchOnce.Do(func() { watchActivations(conn) })
	return nil
}

// deliver sends one notification via freedesktop D-Bus.
func deliver(title, body string, onClick func()) error {
	return sendDBus(title, body, onClick)
}

// watchActivations listens for Activated signals forever; clicking the
// latest notification opens its URL. The daemon outlives every cycle,
// so the goroutine needs no teardown.
func watchActivations(conn *dbus.Conn) {
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(dbusIface),
		dbus.WithMatchMember("Activated"),
	); err != nil {
		log.Printf("notify: match Activated: %v", err)
		return
	}
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	go func() {
		for sig := range ch {
			if len(sig.Body) < 2 {
				continue
			}
			id, _ := sig.Body[0].(uint32)
			action, _ := sig.Body[1].(string)
			if id != 0 && id == lastID.Load() && action == "default" {
				if click := lastClick.Load(); click != nil && *click != nil {
					(*click)()
				}
			}
		}
	}()
}
