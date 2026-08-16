package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/linkedin"
	"github.com/udit-001/waypoint/web"
)

// muxWithFakeLinkedIn builds a mux whose import route is served by a
// linkedin.Fetcher whose callTool seam is stubbed — the same seam package
// tests cross, so the handler test stays honest without a separate fake.
func muxWithFakeLinkedIn(t *testing.T, store db.Store, callTool func(ctx context.Context, tool string, args map[string]any) (string, error)) http.Handler {
	t.Helper()
	staticFS, err := fs.Sub(web.Files, "dist")
	if err != nil {
		t.Fatalf("sub dist: %v", err)
	}
	return newMuxWithLinkedIn(store, staticFS, linkedin.New(linkedin.WithCallTool(callTool)))
}

// importFixture mirrors the parse fixture in internal/linkedin — a realistic
// Exa fetch of a public profile.
const importFixture = `# Jane Doe
URL: https://www.linkedin.com/in/janedoe

# Jane Doe

Senior Software Engineer at Acme Corp

London, United Kingdom

## Experience

### Senior Software Engineer - [Acme Corp](https://www.linkedin.com/company/acme) (Current)

Jan 2020 - Present (6 years)

Built the payments platform.

Department: Engineering

### Staff Engineer - [Globex](https://www.linkedin.com/company/globex)

Jun 2024 - Present (2 years)

Led the platform migration.

## Education

### BS Computer Science at [MIT](https://www.linkedin.com/school/mit)

2012 - 2016 (4 years) in Cambridge, Massachusetts, United States

## Skills

Go • React • distributed systems
`

func importLinkedIn(t *testing.T, mux http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/profile/import-linkedin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// importResult is the {doc, summary} response shape.
type importResult struct {
	Doc     map[string]any        `json:"doc"`
	Summary linkedin.MergeSummary `json:"summary"`
}

func TestImportLinkedInReturnsDocAndSummary(t *testing.T) {
	mux := muxWithFakeLinkedIn(t, db.NewFakeStore(), func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return importFixture, nil
	})

	rec := importLinkedIn(t, mux, `{"url":"https://www.linkedin.com/in/janedoe"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var res importResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	doc := res.Doc
	if doc["name"] != "Jane Doe" || doc["title"] != "Senior Software Engineer at Acme Corp" || doc["currentLocation"] != "London, United Kingdom" {
		t.Errorf("doc head = %v", doc)
	}
	// Empty stored profile → seed: everything lands in the "added" lists.
	if len(res.Summary.ExperienceAdded) != 2 || res.Summary.ExperienceKept != 0 {
		t.Errorf("seed summary = %+v", res.Summary)
	}
	if len(res.Summary.EducationAdded) != 1 || len(res.Summary.SkillsAdded) != 3 {
		t.Errorf("seed summary = %+v", res.Summary)
	}
}

func TestImportLinkedInMergesIntoStoredProfile(t *testing.T) {
	store := db.NewFakeStore()
	// Existing profile: one role LinkedIn also has (but with a manual
	// description), one role LinkedIn does not have, one education entry.
	store.Profile = db.Profile{
		Name:       "Jane Doe",
		Title:      "Old Title",
		Skills:     `["Go"]`,
		Experience: `[{"title":"Senior Software Engineer","company":"Acme Corp","start":"2020-01","end":"","description":"hand written"}]`,
		Education:  `[{"institution":"MIT","degree":"BS CS","start":"2012","end":"2016"}]`,
	}
	mux := muxWithFakeLinkedIn(t, store, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return importFixture, nil
	})

	rec := importLinkedIn(t, mux, `{"url":"https://www.linkedin.com/in/janedoe"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var res importResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Matched role updated (title+company match), manual description replaced
	// by the LinkedIn one; new Globex role added; no deletions.
	exp, _ := res.Doc["experience"].([]any)
	if len(exp) != 2 {
		t.Fatalf("merged experience = %v, want 2 entries", exp)
	}
	if len(res.Summary.ExperienceAdded) != 1 || len(res.Summary.ExperienceUpdated) != 1 || res.Summary.ExperienceKept != 0 {
		t.Errorf("experience summary = %+v", res.Summary)
	}
	// Education matched by institution, degree updated from LinkedIn.
	if len(res.Summary.EducationUpdated) != 1 || res.Summary.EducationKept != 0 {
		t.Errorf("education summary = %+v", res.Summary)
	}
	// Skills: "Go" already present (case-insensitive) → only React + distributed systems added.
	if len(res.Summary.SkillsAdded) != 2 {
		t.Errorf("skills added = %v, want [React distributed systems]", res.Summary.SkillsAdded)
	}
	// Title follows LinkedIn.
	if res.Doc["title"] != "Senior Software Engineer at Acme Corp" {
		t.Errorf("merged title = %v", res.Doc["title"])
	}
}

// TestImportLinkedInRoundTripsThroughPatch is the load-bearing test: the doc
// the import endpoint emits must be accepted verbatim by the existing PATCH
// /api/profile validation (the db seam). If the doc shape ever drifts from the
// profile document spec, this catches it — the web UI applies the import by
// PATCHing exactly this doc.
func TestImportLinkedInRoundTripsThroughPatch(t *testing.T) {
	store := db.NewFakeStore()
	mux := muxWithFakeLinkedIn(t, store, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return importFixture, nil
	})

	rec := importLinkedIn(t, mux, `{"url":"https://www.linkedin.com/in/janedoe"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("import status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var res importResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	docJSON, err := json.Marshal(res.Doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}

	// PATCH the exact doc back through the same mux.
	patch := httptest.NewRequest("PATCH", "/api/profile", strings.NewReader(string(docJSON)))
	patch.Header.Set("Content-Type", "application/json")
	prec := httptest.NewRecorder()
	mux.ServeHTTP(prec, patch)
	if prec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200; body=%s", prec.Code, prec.Body.String())
	}

	p, _ := store.GetProfile()
	if p.Name != "Jane Doe" {
		t.Errorf("stored name = %q", p.Name)
	}
	if p.Experience == "" || !strings.Contains(p.Experience, "Staff Engineer") {
		t.Errorf("stored experience = %q", p.Experience)
	}
	if p.Skills == "" || !strings.Contains(p.Skills, "Go") {
		t.Errorf("stored skills = %q", p.Skills)
	}
}

func TestImportLinkedInRejectsBadURL(t *testing.T) {
	mux := muxWithFakeLinkedIn(t, db.NewFakeStore(), func(_ context.Context, _ string, _ map[string]any) (string, error) {
		t.Fatal("callTool must not run for an invalid URL")
		return "", nil
	})
	for _, body := range []string{
		`{"url":"https://example.com/in/x"}`,
		`{"url":"https://www.linkedin.com/company/acme"}`,
		`{"url":""}`,
	} {
		rec := importLinkedIn(t, mux, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body=%s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestImportLinkedInFetchFailure(t *testing.T) {
	mux := muxWithFakeLinkedIn(t, db.NewFakeStore(), func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "", errors.New("exa fetch: rate limited")
	})
	rec := importLinkedIn(t, mux, `{"url":"https://www.linkedin.com/in/janedoe"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
}

func TestImportLinkedInEmptyProfile(t *testing.T) {
	mux := muxWithFakeLinkedIn(t, db.NewFakeStore(), func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "Sign in to view this profile", nil
	})
	rec := importLinkedIn(t, mux, `{"url":"https://www.linkedin.com/in/janedoe"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rec.Code, rec.Body.String())
	}
}

func TestImportLinkedInBadJSON(t *testing.T) {
	mux := muxWithFakeLinkedIn(t, db.NewFakeStore(), func(_ context.Context, _ string, _ map[string]any) (string, error) {
		t.Fatal("callTool must not run for malformed JSON")
		return "", nil
	})
	rec := importLinkedIn(t, mux, `{`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
