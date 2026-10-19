package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"

	// Activate the compile-time registry the same way the CLI does —
	// the endpoint reads scraper.All(), which is empty without these.
	_ "github.com/udit-001/waypoint/internal/scraper/google"
	_ "github.com/udit-001/waypoint/internal/scraper/iisc"
	_ "github.com/udit-001/waypoint/internal/scraper/indeed"
	_ "github.com/udit-001/waypoint/internal/scraper/linkedin"
	_ "github.com/udit-001/waypoint/internal/scraper/ncbs"
)

// WP-183 Sources tab: the scrapers endpoint lists the compile-time
// registry with each scraper's opt-out state from settings.

func TestScrapersRoute(t *testing.T) {
	f := db.NewFakeStore()
	if err := f.UpsertSettings(map[string]any{
		"autopilot_disabled_scrapers": db.DisabledScrapersJSON([]string{"linkedin"}),
	}); err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/scrapers", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}
	var got struct {
		Scrapers []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Aggregator bool   `json:"aggregator"`
			Disabled   bool   `json:"disabled"`
		} `json:"scrapers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Scrapers) == 0 {
		t.Fatal("no scrapers returned — registry empty?")
	}

	byID := map[string]bool{}
	hasAgg := false
	for _, s := range got.Scrapers {
		if s.ID == "" || s.Name == "" {
			t.Errorf("scraper with empty id or name: %+v", s)
		}
		byID[s.ID] = s.Disabled
		if s.Aggregator {
			hasAgg = true
		}
	}
	if !byID["linkedin"] {
		t.Error("linkedin not marked disabled")
	}
	if byID["iisc"] {
		t.Error("iisc marked disabled, want enabled")
	}
	if !hasAgg {
		t.Error("no aggregator flagged — linkedin/indeed/google must carry the aggregator chip")
	}
}

// The disabled list must reflect a PATCH made after the store was seeded.
func TestScrapersRouteReflectsPatch(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	patchBody, _ := json.Marshal(map[string]any{"autopilot_disabled_scrapers": []string{"indeed"}})
	patch := httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(string(patchBody)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, patch)
	if w.Code != 200 {
		t.Fatalf("PATCH status = %d (%s), want 200", w.Code, w.Body.String())
	}

	req := httptest.NewRequest("GET", "/api/scrapers", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req)

	var got struct {
		Scrapers []struct {
			ID       string `json:"id"`
			Disabled bool   `json:"disabled"`
		} `json:"scrapers"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, s := range got.Scrapers {
		if s.ID == "indeed" && !s.Disabled {
			t.Error("indeed not disabled after PATCH")
		}
	}
}
