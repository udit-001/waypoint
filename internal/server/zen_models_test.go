package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// Handler tests cross the public endpoint with an httptest CDN stand-in
// plugged through the zenCatalogURL seam.

// TestZenModels_curatedCatalog: the endpoint serves the curated catalog
// with friendly names and API families, whatever the stored key.
func TestZenModels_curatedCatalog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"count": 2,
			"models": [
				{"id": "big-pickle", "name": "Big Pickle", "api": "openai-completions"},
				{"id": "union-alpha-free", "name": "Union Alpha Free", "api": "anthropic-messages"}
			]
		}`))
	}))
	defer upstream.Close()
	orig := zenCatalogURL
	zenCatalogURL = upstream.URL
	t.Cleanup(func() { zenCatalogURL = orig })

	// A stored key or not makes no difference: the CDN fetch is anonymous.
	f := db.NewFakeStore()
	f.Settings.ZenAPIKey = "sk-test"
	mux := newMuxWithLinkedIn(f, nil, nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/zen/models", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var out struct {
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			API  string `json:"api"`
		} `json:"models"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Source != "cdn" {
		t.Errorf("source = %q, want cdn", out.Source)
	}
	if len(out.Models) != 2 || out.Models[0].ID != "big-pickle" || out.Models[0].Name != "Big Pickle" || out.Models[1].API != "anthropic-messages" {
		t.Errorf("models = %+v, want named curated entries", out.Models)
	}
}

// TestZenModels_cdnUnreachable: cold cache + dead CDN — nothing to serve.
// The endpoint answers 502 and the web dropdown degrades to the saved
// selection.
func TestZenModels_cdnUnreachable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()
	orig := zenCatalogURL
	zenCatalogURL = upstream.URL
	t.Cleanup(func() { zenCatalogURL = orig })

	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/zen/models", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

// TestZenModels_noStaticFallback: no model list ships in the codebase —
// the endpoint never serves a hardcoded list, only the CDN catalog.
func TestZenModels_noStaticFallback(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Write([]byte(`{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`))
	}))
	defer upstream.Close()
	orig := zenCatalogURL
	zenCatalogURL = upstream.URL
	t.Cleanup(func() { zenCatalogURL = orig })

	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/zen/models", nil))
		if w.Code != 200 {
			t.Fatalf("call %d: status = %d", i, w.Code)
		}
		var out struct {
			Source string `json:"source"`
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		if out.Source != "cdn" {
			t.Fatalf("call %d: source = %q, want cdn", i, out.Source)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 1 {
		t.Errorf("upstream hits = %d, want 1 (SWR cache, no static list)", hits)
	}
}
