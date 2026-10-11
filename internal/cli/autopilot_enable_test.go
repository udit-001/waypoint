package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/db/dbtest"
)

// WP-158: `waypoint autopilot enable|disable` — the CLI adapter over the
// enable contract. The verb is how an agent driving the CLI completes the
// scripted setup path; before this, enabling was web-only.

func TestAutopilotEnable_gateAndRun(t *testing.T) {
	f := db.NewFakeStore()
	store = f
	defer func() { store = nil }()

	// Incomplete brief: refused, nothing flipped.
	jsonOut = true
	defer func() { jsonOut = false }()
	err := autopilotEnableCmd.RunE(autopilotEnableCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "curation brief") {
		t.Fatalf("err = %v, want the brief-complete gate", err)
	}
	if f.Settings.AutopilotEnabled == 1 {
		t.Error("refused enable flipped the toggle")
	}

	// Complete brief: enabled + run requested + valid JSON.
	f.Profile = dbtest.PIIProfile()
	f.Profile.LocationPref = `["Bengaluru"]`
	f.Profile.Companies = `["Stripe"]`
	f.Profile.Keywords = `["distributed systems"]`

	out := captureStdout(t, func() {
		if err := autopilotEnableCmd.RunE(autopilotEnableCmd, nil); err != nil {
			t.Fatalf("enable: %v", err)
		}
	})
	var got struct {
		Enabled      bool `json:"enabled"`
		Cadence      int  `json:"cadence"`
		RunRequested bool `json:"runRequested"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json: %v (%q)", err, out)
	}
	if !got.Enabled || !got.RunRequested {
		t.Errorf("json = %+v, want enabled + runRequested", got)
	}
	if f.Settings.AutopilotEnabled != 1 {
		t.Errorf("AutopilotEnabled = %d, want 1", f.Settings.AutopilotEnabled)
	}
	requested, _ := f.ConsumeAutopilotRunRequest()
	if !requested {
		t.Error("enable did not request an immediate run")
	}

	// --cadence lands in settings.
	autopilotEnableFlags.cadence = 12
	defer func() { autopilotEnableFlags.cadence = 0 }()
	captureStdout(t, func() {
		if err := autopilotEnableCmd.RunE(autopilotEnableCmd, nil); err != nil {
			t.Fatalf("enable --cadence: %v", err)
		}
	})
	if f.Settings.AutopilotCadence != 12 {
		t.Errorf("cadence = %d, want 12", f.Settings.AutopilotCadence)
	}
}

func TestAutopilotDisable(t *testing.T) {
	f := db.NewFakeStore()
	f.Settings.AutopilotEnabled = 1
	f.Settings.AutopilotCadence = 6
	store = f
	jsonOut = true
	defer func() {
		store = nil
		jsonOut = false
	}()

	out := captureStdout(t, func() {
		if err := autopilotDisableCmd.RunE(autopilotDisableCmd, nil); err != nil {
			t.Fatalf("disable: %v", err)
		}
	})
	var got struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json: %v (%q)", err, out)
	}
	if got.Enabled {
		t.Error("json reports enabled")
	}
	if f.Settings.AutopilotEnabled != 0 {
		t.Errorf("AutopilotEnabled = %d, want 0", f.Settings.AutopilotEnabled)
	}
	if f.Settings.AutopilotCadence != 6 {
		t.Errorf("cadence = %d, want untouched", f.Settings.AutopilotCadence)
	}
}
