package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/web"
)

// muxFor builds a mux around a specific store (tests that need a seeded
// FakeStore), unlike newTestMux which always uses a fresh empty one.
func muxFor(t *testing.T, store db.Store) http.Handler {
	t.Helper()
	staticFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		t.Fatalf("sub dist: %v", err)
	}
	return newMux(store, staticFS)
}

func patchProfile(t *testing.T, mux http.Handler, body string) (*httptest.ResponseRecorder, db.Brief) {
	t.Helper()
	req := httptest.NewRequest("PATCH", "/api/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var b db.Brief
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatalf("unmarshal brief: %v", err)
		}
	}
	return rec, b
}

func TestUpdateProfileWritesBrief(t *testing.T) {
	store := db.NewFakeStore()
	mux := muxFor(t, store)

	body := `{"visaSponsorship":"yes","remote":"remote","companies":[" GoLang ","Acme"],"salaryFloor":[{"region":"IN","amount":100000}]}`
	rec, b := patchProfile(t, mux, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if b.Constraints.VisaSponsorship != "yes" {
		t.Errorf("visa_sponsorship = %q, want %q", b.Constraints.VisaSponsorship, "yes")
	}
	if b.Preferences.Remote != "remote" {
		t.Errorf("remote = %q, want %q", b.Preferences.Remote, "remote")
	}
	// List values are normalized to the match form at storage (case-fold, trim).
	wantCompanies := []string{"golang", "acme"}
	if !slices.Equal(b.Preferences.Companies, wantCompanies) {
		t.Errorf("companies = %v, want %v", b.Preferences.Companies, wantCompanies)
	}
	if len(b.Constraints.SalaryFloor) != 1 {
		t.Fatalf("salary_floor = %v, want 1 entry", b.Constraints.SalaryFloor)
	}
	e := b.Constraints.SalaryFloor[0]
	if e.Region != "IN" || e.Amount != 100000 || e.Currency != "INR" {
		t.Errorf("salary_floor entry = %+v, want {IN 100000 INR}", e)
	}
}

func TestUpdateProfileClearsPreference(t *testing.T) {
	store := db.NewFakeStore()
	store.Profile.Companies = `["acme"]`
	mux := muxFor(t, store)

	// An empty array clears the preference (chip removed → set → open).
	rec, b := patchProfile(t, mux, `{"companies":[]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(b.Preferences.Companies) != 0 {
		t.Errorf("companies = %v, want cleared", b.Preferences.Companies)
	}
	if !slices.Contains(b.Open, "companies") {
		t.Errorf("companies should be open after clearing, open = %v", b.Open)
	}
}

func TestUpdateProfileRejectsUnknownField(t *testing.T) {
	mux := muxFor(t, db.NewFakeStore())

	rec, _ := patchProfile(t, mux, `{"bogus":"x"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unknown profile field") {
		t.Errorf("body = %s, want unknown-field error", rec.Body.String())
	}
}

func TestUpdateProfileWritesProfileFields(t *testing.T) {
	store := db.NewFakeStore()
	mux := muxFor(t, store)

	body := `{"name":"Jane Doe","email":"jane@example.com","phone":"+1-555-0123","industry":"biotech"}`
	rec, _ := patchProfile(t, mux, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	p, _ := store.GetProfile()
	if p.Name != "Jane Doe" || p.Email != "jane@example.com" || p.Phone != "+1-555-0123" || p.Industry != "biotech" {
		t.Errorf("profile fields not written: %+v", p)
	}
}

func TestUpdateProfileWritesStructuredExperience(t *testing.T) {
	store := db.NewFakeStore()
	mux := muxFor(t, store)

	body := `{"experience":[{"title":"Senior SWE","company":"Acme","start":"2019-01","end":"2023-06"}],"education":[{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019"}]}`
	rec, _ := patchProfile(t, mux, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	p, _ := store.GetProfile()
	if p.Experience != `[{"title":"Senior SWE","company":"Acme","start":"2019-01","end":"2023-06"}]` {
		t.Errorf("Experience = %q", p.Experience)
	}
	if p.Education != `[{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019"}]` {
		t.Errorf("Education = %q", p.Education)
	}
	// Derived seniority now comes from the date range, not regex.
	b, _ := store.GetBrief()
	if b.Facts.Seniority != "mid" {
		t.Errorf("seniority = %q, want mid (from date range)", b.Facts.Seniority)
	}
}

func TestUpdateProfileRejectsBadStructured(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"experience missing title", `{"experience":[{"company":"Acme"}]}`, "title is required"},
		{"experience bad date", `{"experience":[{"title":"SWE","start":"03/2021"}]}`, "invalid date"},
		{"education missing institution", `{"education":[{"degree":"BS"}]}`, "institution is required"},
		{"education bad end date", `{"education":[{"institution":"MIT","end":"2021-13"}]}`, "invalid date"},
		{"experience not an array", `{"experience":{"title":"SWE"}}`, "must be a JSON array"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := muxFor(t, db.NewFakeStore())
			rec, _ := patchProfile(t, mux, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantErr) {
				t.Errorf("body = %s, want error containing %q", rec.Body.String(), tc.wantErr)
			}
		})
	}
}

func TestUpdateProfileRejectsBadValues(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"malformed json", `{`, "invalid JSON"},
		{"empty body", ``, "no fields"},
		{"non-string scalar", `{"remote":5}`, "expected a string"},
		{"non-array list", `{"companies":"acme"}`, "array of strings"},
		{"negative salary", `{"salaryFloor":[{"region":"IN","amount":-1}]}`, "positive"},
		{"missing salary region", `{"salaryFloor":[{"amount":1000}]}`, "region"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := muxFor(t, db.NewFakeStore())
			rec, _ := patchProfile(t, mux, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantErr) {
				t.Errorf("body = %s, want error containing %q", rec.Body.String(), tc.wantErr)
			}
		})
	}
}

// TestUpdateProfileRejectsSnakeKeys: the PATCH payload uses the profile
// document keys (camelCase, matching GET /api/profile and the CLI doc). The
// old snake_case keys are unknown fields now — a typo, never a silent drop.
func TestUpdateProfileRejectsSnakeKeys(t *testing.T) {
	for _, key := range []string{"visa_sponsorship", "salary_floor", "current_location", "location_preference", "avoid_companies"} {
		t.Run(key, func(t *testing.T) {
			mux := muxFor(t, db.NewFakeStore())
			rec, _ := patchProfile(t, mux, `{"`+key+`":"x"}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "unknown profile field") {
				t.Errorf("body = %s, want unknown-field error", rec.Body.String())
			}
		})
	}
}

func TestUpdateProfileSeniorityGate(t *testing.T) {
	t.Run("derived from experience blocks manual set", func(t *testing.T) {
		store := db.NewFakeStore()
		store.Profile.Experience = `["5 years backend development"]`
		mux := muxFor(t, store)

		rec, _ := patchProfile(t, mux, `{"seniority":"mid"}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "derives from experience") {
			t.Errorf("body = %s, want derive-gate error", rec.Body.String())
		}
	})

	t.Run("allowed before experience arrives", func(t *testing.T) {
		mux := muxFor(t, db.NewFakeStore())
		rec, b := patchProfile(t, mux, `{"seniority":"junior"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		if b.Facts.Seniority != "junior" {
			t.Errorf("seniority = %q, want %q", b.Facts.Seniority, "junior")
		}
	})
}

func TestGetBrief(t *testing.T) {
	store := db.NewFakeStore()
	store.Profile.CurrentLocation = "Bengaluru"
	store.Profile.Remote = "remote"
	mux := muxFor(t, store)

	req := httptest.NewRequest("GET", "/api/brief", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var b db.Brief
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if b.Facts.CurrentLocation != "Bengaluru" {
		t.Errorf("current_location = %q, want %q", b.Facts.CurrentLocation, "Bengaluru")
	}
	if b.Preferences.Remote != "remote" {
		t.Errorf("remote = %q, want %q", b.Preferences.Remote, "remote")
	}
	if b.Complete {
		t.Error("brief should be incomplete: preferences are open")
	}
	// The open frontier lists the unsettled preferences.
	for _, want := range []string{"location_preference", "companies", "keywords"} {
		if !slices.Contains(b.Open, want) {
			t.Errorf("open = %v, want it to contain %q", b.Open, want)
		}
	}
}
