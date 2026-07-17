package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/web"
)

// newTestMux creates a mux with the real embedded static FS and a fake store.
// This tests the actual route configuration, not a subset.
func newTestMux(t *testing.T) http.Handler {
	t.Helper()
	staticFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		t.Fatalf("sub dist: %v", err)
	}
	return newMux(db.NewFakeStore(), staticFS)
}

func TestPWAStaticRoutes(t *testing.T) {
	mux := newTestMux(t)
	cases := []struct {
		name, path, wantContentType string
	}{
		{"service-worker", "/sw.js", "application/javascript"},
		{"manifest", "/manifest.json", "application/manifest+json"},
		{"offline-page", "/offline.html", "text/html"},
		{"icon-192-png", "/icons/icon-192.png", "image/png"},
		{"icon-512-png", "/icons/icon-512.png", "image/png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			if rec.Code != 200 {
				t.Errorf("%s: status = %d, want 200", tc.path, rec.Code)
			}
			ct := rec.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, tc.wantContentType) {
				t.Errorf("%s: Content-Type = %q, want prefix %q", tc.path, ct, tc.wantContentType)
			}
		})
	}
}

func TestPWACacheHeaders(t *testing.T) {
	mux := newTestMux(t)
	cases := []struct {
		name, path, wantCacheControl, wantSWAllowed string
	}{
		{"sw.js no-cache + scope", "/sw.js", "no-cache", "/"},
		{"manifest cached 1h", "/manifest.json", "public, max-age=3600", ""},
		{"offline no-cache", "/offline.html", "no-cache", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			cc := rec.Header().Get("Cache-Control")
			if cc != tc.wantCacheControl {
				t.Errorf("%s: Cache-Control = %q, want %q", tc.path, cc, tc.wantCacheControl)
			}
			if tc.wantSWAllowed != "" {
				sw := rec.Header().Get("Service-Worker-Allowed")
				if sw != tc.wantSWAllowed {
					t.Errorf("%s: Service-Worker-Allowed = %q, want %q", tc.path, sw, tc.wantSWAllowed)
				}
			}
		})
	}
}

func TestPWAHeadTags(t *testing.T) {
	mux := newTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()

	checks := []struct{ name, want string }{
		{"manifest link", `rel="manifest" href="/manifest.json"`},
		{"sentinel meta", `<meta name="waypoint-app" content="1"`},
		{"theme-color", `<meta name="theme-color" content="#5E81AC"`},
		{"apple-touch-icon", `rel="apple-touch-icon" href="/icons/icon-192.png"`},
		{"sw registration", "navigator.serviceWorker.register('/sw.js')"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(body, c.want) {
				t.Errorf("served HTML missing %s: want substring %q", c.name, c.want)
			}
		})
	}
}

func TestPWASentinelAgreement(t *testing.T) {
	// The SENTINEL string in sw.js must match the meta tag in index.html.
	// If they drift, the identity guard breaks silently — navigations from
	// a foreign app on the same port would be served instead of the offline page.
	staticFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		t.Fatalf("sub dist: %v", err)
	}

	sentinel := `name="waypoint-app"`

	swData, err := fs.ReadFile(staticFS, "sw.js")
	if err != nil {
		t.Fatalf("read sw.js: %v", err)
	}
	if !strings.Contains(string(swData), sentinel) {
		t.Errorf("sw.js missing SENTINEL string %q", sentinel)
	}

	indexData, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexData), sentinel) {
		t.Errorf("index.html missing sentinel meta tag %q", sentinel)
	}
}
