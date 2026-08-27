package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// With a stored key, the handler proxies the gateway and filters to
// free-tier ids only.
func TestZenModels_liveGatewayFiltered(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing bearer token")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
			{"id": "mimo-v2.5-free"},
			{"id": "deepseek-v4-flash-free"},
			{"id": "claude-haiku-4-5"}, // paid — filtered out
		}})
	}))
	defer upstream.Close()
	orig := zenModelsURL
	zenModelsURL = upstream.URL + "/v1/models"
	t.Cleanup(func() { zenModelsURL = orig })

	f := db.NewFakeStore()
	f.Settings.ZenAPIKey = "sk-test"
	mux := newMuxWithLinkedIn(f, nil, nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/zen/models", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var out struct {
		Models []string `json:"models"`
		Source string   `json:"source"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Source != "gateway" {
		t.Errorf("source = %q, want gateway", out.Source)
	}
	if len(out.Models) != 2 {
		t.Errorf("models = %v, want the two free ids", out.Models)
	}
}

// Without a key: static fallback list, no upstream call.
func TestZenModels_fallbackWithoutKey(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/zen/models", nil))
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var out struct {
		Source string `json:"source"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Source != "fallback" {
		t.Errorf("source = %q, want fallback", out.Source)
	}
}
