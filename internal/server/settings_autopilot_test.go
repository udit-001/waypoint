package server

import (
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
