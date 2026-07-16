package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitForServerReadyTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/stats" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	port := portFromAddr(srv.Listener.Addr().String())
	// Server is already running — should return true quickly
	if !waitForServerReady(port, 20, 10*time.Millisecond) {
		t.Error("waitForServerReady() = false, want true (server is running)")
	}
}

func TestWaitForServerReadyFalse(t *testing.T) {
	// Port 1 is almost certainly not serving anything
	// Use small attempts/interval to keep the test fast
	if waitForServerReady(1, 3, 10*time.Millisecond) {
		t.Error("waitForServerReady(1) = true, want false (nothing on port 1)")
	}
}

func TestWaitForServerReadyNonWaypointServer(t *testing.T) {
	// A server without /api/stats should never be considered ready
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	port := portFromAddr(srv.Listener.Addr().String())
	if waitForServerReady(port, 3, 10*time.Millisecond) {
		t.Error("waitForServerReady() = true for non-waypoint server, want false")
	}
}
