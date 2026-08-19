package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/udit-001/waypoint/internal/db"
)

// handleEvents exposes the live-sync stream. The client subscribes with
// GET /api/events?cursor=<lastSeenChangeID>; the handler tails the
// change_events table (the DB-backed bus — writers may be in another
// process, e.g. a CLI `waypoint autopilot run`) and emits one SSE event
// per new row. Events carry only a "kind"; consumers refetch their own
// state (see web/src/lib/live.js).
//
// Connection hygiene (ported from html-organizer's fe822a1): the server
// is plain HTTP 127.0.0.1 → HTTP/1.1, so the browser caps this origin at
// 6 parallel connections. The client therefore holds the EventSource only
// while its tab is visible — this handler just streams while asked.
func handleEvents(store db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cursor := int64(0)
		if s := r.URL.Query().Get("cursor"); s != "" {
			fmt.Sscanf(s, "%d", &cursor)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // no proxy buffering (localhost insurance)

		// SSE connections are long-lived; the server's global WriteTimeout
		// (60s) would otherwise kill them. Clear the deadline for this
		// connection only.
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Keepalive comment so the client knows the stream is live before
		// anything is broadcast.
		fmt.Fprint(w, ":connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		lastID := cursor
		pollTicker := time.NewTicker(2 * time.Second)
		defer pollTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-pollTicker.C:
				events, err := store.ChangesSince(lastID, 50)
				if err != nil {
					continue // DB hiccup: skip this tick, keep streaming
				}
				for _, ev := range events {
					fmt.Fprintf(w, "data: %s\n\n", marshalEvent(ev))
					if ev.ID > lastID {
						lastID = ev.ID
					}
				}
				if len(events) > 0 {
					flusher.Flush()
				}
			}
		}
	}
}

// marshalEvent renders a ChangeEvent as JSON. Inline (no encoding/json
// in the hot path — it's one small struct).
func marshalEvent(ev db.ChangeEvent) string {
	return fmt.Sprintf(`{"id":%d,"kind":%q}`, ev.ID, ev.Kind)
}
