package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// WP-175: `waypoint scrape disable|enable <name>` persists the scraper
// opt-out through the same db.Store seam as the web, validates the name
// against the scraper registry, and is idempotent on repeat.

func TestScrapeDisableUnknownScraperErrors(t *testing.T) {
	store = db.NewFakeStore()
	jsonOut = false

	err := scrapeDisableCmd.RunE(scrapeDisableCmd, []string{"no-such-scraper"})
	if err == nil || !strings.Contains(err.Error(), "unknown scraper") {
		t.Fatalf("error = %v, want 'unknown scraper ...'", err)
	}
}

func TestScrapeDisableThenEnableIdempotent(t *testing.T) {
	f := db.NewFakeStore()
	store = f
	jsonOut = false

	// Disable twice — second repeat must not duplicate or error.
	for i := 0; i < 2; i++ {
		if err := scrapeDisableCmd.RunE(scrapeDisableCmd, []string{"ncbs"}); err != nil {
			t.Fatalf("disable ncbs (pass %d): %v", i+1, err)
		}
	}
	st, _ := f.GetSettings()
	if got, want := st.AutopilotDisabledScrapers, `["ncbs"]`; got != want {
		t.Fatalf("after 2x disable = %s, want %s", got, want)
	}

	// Disable a second scraper — both persist.
	if err := scrapeDisableCmd.RunE(scrapeDisableCmd, []string{"linkedin"}); err != nil {
		t.Fatalf("disable linkedin: %v", err)
	}
	st, _ = f.GetSettings()
	if got, want := st.AutopilotDisabledScrapers, `["ncbs","linkedin"]`; got != want {
		t.Fatalf("after 2nd disable = %s, want %s", got, want)
	}

	// Enable twice — second repeat is a no-op, not an error.
	for i := 0; i < 2; i++ {
		if err := scrapeEnableCmd.RunE(scrapeEnableCmd, []string{"ncbs"}); err != nil {
			t.Fatalf("enable ncbs (pass %d): %v", i+1, err)
		}
	}
	st, _ = f.GetSettings()
	if got, want := st.AutopilotDisabledScrapers, `["linkedin"]`; got != want {
		t.Errorf("after enable = %s, want %s", got, want)
	}
}

func TestScrapeDisableJSONOutput(t *testing.T) {
	f := db.NewFakeStore()
	store = f
	jsonOut = true
	defer func() { jsonOut = false }()

	var err error
	out := captureStdout(t, func() {
		err = scrapeDisableCmd.RunE(scrapeDisableCmd, []string{"ncbs"})
	})
	if err != nil {
		t.Fatalf("disable: %v", err)
	}

	var result struct {
		Scraper          string   `json:"scraper"`
		Disabled         bool     `json:"disabled"`
		DisabledScrapers []string `json:"disabledScrapers"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if result.Scraper != "ncbs" || !result.Disabled {
		t.Errorf("scraper/disabled = %q/%v, want ncbs/true", result.Scraper, result.Disabled)
	}
	if len(result.DisabledScrapers) != 1 || result.DisabledScrapers[0] != "ncbs" {
		t.Errorf("disabledScrapers = %v, want [ncbs]", result.DisabledScrapers)
	}
}

// The agent surface: autopilot status --json exposes the constraint list so
// an agent can answer "why was this source skipped" without the DB.
func TestAutopilotStatusJSONIncludesDisabledScrapers(t *testing.T) {
	f := db.NewFakeStore()
	if err := f.UpsertSettings(map[string]any{
		"autopilot_disabled_scrapers": `["linkedin","indeed"]`,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	store = f
	jsonOut = true
	defer func() { jsonOut = false }()

	var err error
	out := captureStdout(t, func() {
		err = autopilotStatusCmd.RunE(autopilotStatusCmd, nil)
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	var result struct {
		DisabledScrapers []string `json:"disabledScrapers"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(result.DisabledScrapers) != 2 || result.DisabledScrapers[0] != "linkedin" || result.DisabledScrapers[1] != "indeed" {
		t.Errorf("disabledScrapers = %v, want [linkedin indeed]", result.DisabledScrapers)
	}
}
