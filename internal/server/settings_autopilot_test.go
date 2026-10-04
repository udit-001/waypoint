package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// seedCompleteBrief fills the four preferences the gate requires:
// Complete is true iff remote, location_pref, companies and keywords are
// all settled (internal/db/brief.go).
func seedCompleteBrief(f *db.FakeStore) {
	f.Profile.Remote = "hybrid"
	f.Profile.LocationPref = `["Bengaluru"]`
	f.Profile.Companies = `["Stripe"]`
	f.Profile.Keywords = `["backend"]`
}

// Enabling autopilot through the settings API must drop a run-request
// marker — that's the whole "no dead air" contract: the daemon's
// scheduler consumes it within seconds instead of waiting a cadence.
func TestUpdateSettingsEnableAutopilotRequestsRun(t *testing.T) {
	f := db.NewFakeStore()
	seedCompleteBrief(f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("PATCH", "/api/settings",
		strings.NewReader(`{"autopilot_enabled": 1}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}

	requested, err := f.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if !requested {
		t.Fatal("enabling autopilot did not request a run")
	}
}

// Disabling must NOT request a run.
func TestUpdateSettingsDisableAutopilotNoRunRequest(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("PATCH", "/api/settings",
		strings.NewReader(`{"autopilot_enabled": 0}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	requested, _ := f.ConsumeAutopilotRunRequest()
	if requested {
		t.Error("disabling autopilot requested a run")
	}
}

// A settings PATCH that doesn't touch the toggle leaves no marker.
func TestUpdateSettingsOtherFieldsNoRunRequest(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("PATCH", "/api/settings",
		strings.NewReader(`{"theme": "dark"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	requested, _ := f.ConsumeAutopilotRunRequest()
	if requested {
		t.Error("theme change requested an autopilot run")
	}
}

// The Settings UI's Exa card PATCHes exa_api_key — the handler must accept
// it (it was missing from the allow-list, so saving a key from the web 400'd)
// and the saved key must round-trip into settings where discovery and the
// LinkedIn import resolver read it.
func TestUpdateSettingsAcceptsExaAPIKey(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("PATCH", "/api/settings",
		strings.NewReader(`{"exa_api_key": "exa-test-key"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}
	stored, err := f.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if stored.ExaAPIKey != "exa-test-key" {
		t.Errorf("stored ExaAPIKey = %q, want exa-test-key", stored.ExaAPIKey)
	}
}

// WP-175: the scraper opt-out list round-trips through PATCH /api/settings
// (JSON array in, canonical JSON array string in the store) and comes back
// as an array on GET — the shape the Sources tab and the Zen selection
// constraint both read.
func TestUpdateSettingsDisabledScrapersRoundTrip(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("PATCH", "/api/settings",
		strings.NewReader(`{"autopilot_disabled_scrapers": ["linkedin", "ncbs"]}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("PATCH status = %d (%s), want 200", w.Code, w.Body.String())
	}

	stored, err := f.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got, want := stored.AutopilotDisabledScrapers, `["linkedin","ncbs"]`; got != want {
		t.Errorf("stored = %s, want %s", got, want)
	}

	get := httptest.NewRequest("GET", "/api/settings", nil)
	gw := httptest.NewRecorder()
	mux.ServeHTTP(gw, get)
	if gw.Code != 200 {
		t.Fatalf("GET status = %d, want 200", gw.Code)
	}
	var view struct {
		Disabled []string `json:"autopilotDisabledScrapers"`
	}
	if err := json.Unmarshal(gw.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode GET body: %v\nbody: %s", err, gw.Body.String())
	}
	if len(view.Disabled) != 2 || view.Disabled[0] != "linkedin" || view.Disabled[1] != "ncbs" {
		t.Errorf("GET autopilotDisabledScrapers = %v, want [linkedin ncbs]", view.Disabled)
	}
}

// The opt-out is a hard constraint — a malformed payload must fail loudly,
// never silently leave the constraint unset: a non-array fails, and so does
// an array carrying non-string items.
func TestUpdateSettingsDisabledScrapersRejectsNonArray(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	for _, body := range []string{
		`{"autopilot_disabled_scrapers": "linkedin"}`,
		`{"autopilot_disabled_scrapers": ["linkedin", 5]}`,
	} {
		req := httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("PATCH %s: status = %d (%s), want 400", body, w.Code, w.Body.String())
		}
	}
	stored, _ := f.GetSettings()
	if stored.AutopilotDisabledScrapers != "" {
		t.Errorf("store changed on rejected payload: %q", stored.AutopilotDisabledScrapers)
	}
}

// discovery_interval_days is a whitelisted settings key (the Sources tab
// patches it); it was missing from the handler, so the PATCH 400'd.
func TestUpdateSettingsAcceptsDiscoveryInterval(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("PATCH", "/api/settings",
		strings.NewReader(`{"discovery_interval_days": 14}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}
	stored, err := f.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if stored.DiscoveryIntervalDays != 14 {
		t.Errorf("DiscoveryIntervalDays = %d, want 14", stored.DiscoveryIntervalDays)
	}
}
