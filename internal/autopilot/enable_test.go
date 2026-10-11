package autopilot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/db/dbtest"
)

// The enable contract (WP-158): one gate + one write + one run request,
// shared by the web handler and the CLI. Enabling with an incomplete brief
// is the refused path; enabling asks the daemon for an immediate cycle so
// matches arrive in seconds, not after a full cadence.

func TestEnable_gateRequiresCompleteBrief(t *testing.T) {
	f := db.NewFakeStore()
	// Fresh FakeStore: no profile, so the brief derives incomplete.

	err := Enable(context.Background(), f, 0)
	if err == nil || !strings.Contains(err.Error(), "curation brief") {
		t.Fatalf("err = %v, want the brief-complete gate", err)
	}
	if f.Settings.AutopilotEnabled == 1 {
		t.Error("gate refused but the toggle flipped")
	}
	var requested bool
	requested, _ = f.ConsumeAutopilotRunRequest()
	if requested {
		t.Error("refused enable still requested a run")
	}
}

func TestEnable_persistsAndRequestsRun(t *testing.T) {
	f := db.NewFakeStore()
	completeBrief(t, f)

	if err := Enable(context.Background(), f, 0); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if f.Settings.AutopilotEnabled != 1 {
		t.Errorf("AutopilotEnabled = %d, want 1", f.Settings.AutopilotEnabled)
	}
	requested, _ := f.ConsumeAutopilotRunRequest()
	if !requested {
		t.Error("enable did not request an immediate run")
	}
	// Cadence 0 means "leave the stored cadence alone".
	if f.Settings.AutopilotCadence != 6 {
		t.Errorf("cadence = %d, want the stored default untouched", f.Settings.AutopilotCadence)
	}
}

func TestEnable_withCadence(t *testing.T) {
	f := db.NewFakeStore()
	completeBrief(t, f)

	if err := Enable(context.Background(), f, 12); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if f.Settings.AutopilotCadence != 12 {
		t.Errorf("cadence = %d, want 12", f.Settings.AutopilotCadence)
	}
}

func TestDisable(t *testing.T) {
	f := db.NewFakeStore()
	f.Settings.AutopilotEnabled = 1
	f.Settings.AutopilotCadence = 6

	if err := Disable(f); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if f.Settings.AutopilotEnabled != 0 {
		t.Errorf("AutopilotEnabled = %d, want 0", f.Settings.AutopilotEnabled)
	}
	if f.Settings.AutopilotCadence != 6 {
		t.Errorf("cadence = %d, want untouched (disable never rewrites it)", f.Settings.AutopilotCadence)
	}
	// Disabling must not queue a run — nothing is enabled to consume it.
	requested, _ := f.ConsumeAutopilotRunRequest()
	if requested {
		t.Error("disable requested a run")
	}
}

func TestEnable_briefReadFailure(t *testing.T) {
	f := db.NewFakeStore()
	// briefGetter failure surfaces as the gate refusing, not a panic.
	if err := Enable(context.Background(), errBriefStore{f}, 0); err == nil {
		t.Fatal("want the gate to surface the read failure")
	}
}

func completeBrief(t *testing.T, f *db.FakeStore) {
	t.Helper()
	// The brief derives from the profile; Complete means the essential
	// search intent (location, companies, keywords) is settled.
	f.Profile = dbtest.PIIProfile()
	f.Profile.LocationPref = `["Bengaluru"]`
	f.Profile.Companies = `["Stripe"]`
	f.Profile.Keywords = `["distributed systems"]`
	brief, err := f.GetBrief()
	if err != nil || !brief.Complete {
		t.Fatalf("seed brief not complete: complete=%v err=%v", brief.Complete, err)
	}
}

type errBriefStore struct {
	db.Store
}

func (e errBriefStore) GetBrief() (db.Brief, error) {
	return db.Brief{}, errors.New("disk on fire")
}
