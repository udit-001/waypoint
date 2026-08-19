package server

import (
	"bufio"
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/web"
)

// TestEventsStreamsChangeEvents connects to the SSE endpoint, waits for
// the connected comment, publishes a change event (simulating an
// in-flight autopilot cycle publishing from another process), and
// asserts the event arrives as an SSE data line.
func TestEventsStreamsChangeEvents(t *testing.T) {
	fake := db.NewFakeStore()
	staticFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		t.Fatalf("sub dist: %v", err)
	}
	mux := newMux(fake, staticFS)

	// Use a real httptest server — SSE needs a real connection for the
	// ResponseController/FluISher to behave like production.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events?cursor=0", nil)
	client := srv.Client()

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	br := bufio.NewReader(resp.Body)

	readComment := func() string {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read comment: %v", err)
		}
		return line
	}

	// First line is the :connected comment.
	if got := readComment(); !strings.HasPrefix(got, ":connected") {
		t.Fatalf("first line = %q, want :connected comment", got)
	}

	// Publish a change event after subscribe.
	if err := fake.AddChangeEvent("matches"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Read lines in a dedicated goroutine; the poll runs every 2s so
	// allow ~5s for the event to surface.
	lines := make(chan string, 8)
	go func() {
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				close(lines)
				return
			}
			lines <- line
		}
	}()

	deadline := time.After(6 * time.Second)
	var found bool
	for !found {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before event")
			}
			if strings.HasPrefix(line, "data: ") {
				if !strings.Contains(line, `"kind":"matches"`) {
					t.Fatalf("unexpected event: %q", line)
				}
				found = true
			}
		case <-deadline:
			t.Fatal("timeout: never received the 'matches' event")
		}
	}
}
