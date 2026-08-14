package db

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestNormalizeProfileDocument exercises the patch path: only present keys are
// normalized into the store-ready map, in the shape the Store seam expects.
func TestNormalizeProfileDocument(t *testing.T) {
	doc := map[string]json.RawMessage{
		"name":            json.RawMessage(`"Jane Doe"`),
		"currentLocation": json.RawMessage(`"Bengaluru"`),
		"skills":          json.RawMessage(`["Go","React"]`),
	}
	updates, err := NormalizeProfileDocument(doc, "")
	if err != nil {
		t.Fatalf("NormalizeProfileDocument: %v", err)
	}
	if updates["name"] != "Jane Doe" {
		t.Errorf("name = %#v, want Jane Doe", updates["name"])
	}
	// Doc keys are camelCase (read surface); the updates map uses the Store
	// seam's snake_case keys.
	if updates["current_location"] != "Bengaluru" {
		t.Errorf("current_location = %#v", updates["current_location"])
	}
	// Lists pass through as a JSON-array string; the Store normalizes to
	// match form on write.
	if updates["skills"] != `["Go","React"]` {
		t.Errorf("skills = %#v, want [\"Go\",\"React\"]", updates["skills"])
	}
}

// TestNormalizeProfileDocumentPatchSemantics: keys absent from the doc never
// appear in the updates map (the caller upserts only present keys).
func TestNormalizeProfileDocumentPatchSemantics(t *testing.T) {
	updates, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"title": json.RawMessage(`"Senior Engineer"`),
	}, "")
	if err != nil {
		t.Fatalf("NormalizeProfileDocument: %v", err)
	}
	if len(updates) != 1 {
		t.Errorf("updates has %d keys, want 1 (patch semantics)", len(updates))
	}
}

// TestNormalizeProfileDocumentUnknownKey: a typo is a hard error, never a
// silent no-op.
func TestNormalizeProfileDocumentUnknownKey(t *testing.T) {
	_, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"namee": json.RawMessage(`"Jane"`),
	}, "")
	if err == nil || !strings.Contains(err.Error(), `unknown profile field "namee"`) {
		t.Errorf("error = %v, want unknown profile field", err)
	}
}

func TestNormalizeProfileDocumentEmpty(t *testing.T) {
	_, err := NormalizeProfileDocument(map[string]json.RawMessage{}, "")
	if err == nil || !strings.Contains(err.Error(), "no fields provided") {
		t.Errorf("error = %v, want no fields provided", err)
	}
}

// TestNormalizeProfileDocumentTypeErrors: wrong value shapes are rejected with
// the per-kind message.
func TestNormalizeProfileDocumentTypeErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  map[string]json.RawMessage
		want string
	}{
		{"scalar given array", map[string]json.RawMessage{"name": json.RawMessage(`["Jane"]`)}, "expected a string"},
		{"list given string", map[string]json.RawMessage{"skills": json.RawMessage(`"Go"`)}, "expected an array of strings"},
		{"malformed json", map[string]json.RawMessage{"name": json.RawMessage(`oops`)}, "expected a string"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeProfileDocument(tc.doc, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestNormalizeProfileDocumentSalaryFloor(t *testing.T) {
	updates, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"salaryFloor": json.RawMessage(`[{"region":"IN","amount":100000},{"region":"GB","amount":30000}]`),
	}, "")
	if err != nil {
		t.Fatalf("NormalizeProfileDocument: %v", err)
	}
	if updates["salary_floor"] != `[{"region":"IN","amount":100000},{"region":"GB","amount":30000}]` {
		t.Errorf("salary_floor = %#v", updates["salary_floor"])
	}

	for _, tc := range []struct {
		name, raw, want string
	}{
		{"missing region", `[{"amount":100000}]`, "region is required"},
		{"blank region", `[{"region":"  ","amount":100000}]`, "region is required"},
		{"non-positive amount", `[{"region":"IN","amount":0}]`, "amount must be a positive number"},
		{"bad shape", `[1,2]`, "expected an array of {region, amount}"},
		{"not an array", `"IN:100000"`, "expected an array of {region, amount}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeProfileDocument(map[string]json.RawMessage{
				"salaryFloor": json.RawMessage(tc.raw),
			}, "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// TestNormalizeProfileDocumentExperienceEducation: description survives the
// patch path; entry validation errors surface with the key prefix.
func TestNormalizeProfileDocumentExperienceEducation(t *testing.T) {
	updates, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"experience": json.RawMessage(`[{"title":"SWE","company":"Acme","start":"2021-03","description":"Led team of 5"}]`),
		"education":  json.RawMessage(`[{"institution":"MIT","degree":"BS","description":"GPA 3.9"}]`),
	}, "")
	if err != nil {
		t.Fatalf("NormalizeProfileDocument: %v", err)
	}
	wantExp := `[{"title":"SWE","company":"Acme","start":"2021-03","end":"","description":"Led team of 5"}]`
	if updates["experience"] != wantExp {
		t.Errorf("experience = %#v, want %s", updates["experience"], wantExp)
	}
	if got := ParseExperienceEntries(updates["experience"].(string)); len(got) != 1 || got[0].Description != "Led team of 5" {
		t.Errorf("experience description lost: %#v", got)
	}

	_, err = NormalizeProfileDocument(map[string]json.RawMessage{
		"experience": json.RawMessage(`[{"company":"Acme"}]`),
	}, "")
	if err == nil || !strings.Contains(err.Error(), "experience: entry 1: title is required") {
		t.Errorf("error = %v, want experience: entry 1: title is required", err)
	}
}

// TestNormalizeProfileDocumentSeniorityGate: manual seniority is rejected once
// experience carries a derived level; allowed before a resume seed arrives.
func TestNormalizeProfileDocumentSeniorityGate(t *testing.T) {
	// Derived experience (5 years) + manual seniority → rejected.
	_, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"seniority": json.RawMessage(`"mid"`),
	}, `[{"title":"SWE","start":"2019-01","end":"2024-01"}]`)
	if err == nil || !strings.Contains(err.Error(), "seniority derives from experience") {
		t.Errorf("error = %v, want seniority derives from experience", err)
	}

	// No experience yet → manual seniority is the placeholder.
	updates, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"seniority": json.RawMessage(`"mid"`),
	}, "")
	if err != nil {
		t.Fatalf("NormalizeProfileDocument: %v", err)
	}
	if updates["seniority"] != "mid" {
		t.Errorf("seniority = %#v, want mid", updates["seniority"])
	}

	// Experience without a year signal → manual seniority still allowed.
	if _, err := NormalizeProfileDocument(map[string]json.RawMessage{
		"seniority": json.RawMessage(`"mid"`),
	}, `[{"title":"SWE"}]`); err != nil {
		t.Errorf("expected allowed with date-less experience, got %v", err)
	}

	// The gate evaluates the doc's OWN experience: a doc that writes a
	// derived level AND a manual seniority in the same pass is rejected,
	// even when the stored experience was empty.
	_, err = NormalizeProfileDocument(map[string]json.RawMessage{
		"experience": json.RawMessage(`[{"title":"SWE","start":"2019-01","end":"2024-01"}]`),
		"seniority":  json.RawMessage(`"junior"`),
	}, "")
	if err == nil || !strings.Contains(err.Error(), "seniority derives from experience") {
		t.Errorf("error = %v, want seniority derives from experience (doc's own experience)", err)
	}
}

// TestProfileSchemaTemplate: the template is the writable surface — every
// writable key present, entry shapes from the structs, valid JSON.
func TestProfileSchemaTemplate(t *testing.T) {
	tmpl := ProfileSchemaTemplate()

	var doc map[string]any
	if err := json.Unmarshal([]byte(tmpl), &doc); err != nil {
		t.Fatalf("template is not valid JSON: %v\n%s", err, tmpl)
	}

	for _, k := range []string{
		"name", "email", "phone", "title", "industry", "currentLocation",
		"visaSponsorship", "remote", "seniority",
		"skills", "locationPreference", "companies", "avoidCompanies",
		"keywords", "dealbreakers", "salaryFloor", "experience", "education",
	} {
		if _, ok := doc[k]; !ok {
			t.Errorf("template missing writable key %q", k)
		}
	}

	// salary_floor is an empty list in the template — a fill-in entry would
	// fail validation, and the template must round-trip through set.
	if floors, ok := doc["salaryFloor"].([]any); !ok || len(floors) != 0 {
		t.Errorf("salaryFloor template = %#v, want []", doc["salaryFloor"])
	}

	// Entry objects carry the exact struct shape (single source of truth).
	entries, ok := doc["experience"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("experience template = %#v, want one empty entry", doc["experience"])
	}
	entry := entries[0].(map[string]any)
	for _, f := range []string{"title", "company", "start", "end", "description"} {
		if entry[f] != "" {
			t.Errorf("experience entry template %q = %#v, want empty string", f, entry[f])
		}
	}

	edu, ok := doc["education"].([]any)
	if !ok || len(edu) != 1 {
		t.Fatalf("education template = %#v", doc["education"])
	}
	for _, f := range []string{"institution", "degree", "start", "end", "description"} {
		if _, ok := edu[0].(map[string]any)[f]; !ok {
			t.Errorf("education entry template missing %q", f)
		}
	}

	// No derived/read-only keys leak into the writable surface.
	for _, k := range []string{"createdAt", "updatedAt"} {
		if _, ok := doc[k]; ok {
			t.Errorf("template contains non-writable key %q", k)
		}
	}

	// Deterministic output.
	if tmpl != ProfileSchemaTemplate() {
		t.Error("template output is not deterministic")
	}
}

// TestEmptyEntryTemplateShape: the template shape matches the entry structs'
// json tags — if a struct field is added later, this test forces the template
// to follow.
func TestEmptyEntryTemplateShape(t *testing.T) {
	if got := emptyEntryTemplate("experience"); !reflect.DeepEqual(got, map[string]string{
		"title": "", "company": "", "start": "", "end": "", "description": "",
	}) {
		t.Errorf("experience template = %#v", got)
	}
	if got := emptyEntryTemplate("education"); !reflect.DeepEqual(got, map[string]string{
		"institution": "", "degree": "", "start": "", "end": "", "description": "",
	}) {
		t.Errorf("education template = %#v", got)
	}
	if got := emptyEntryTemplate("unknown"); got != nil {
		t.Errorf("unknown key template = %#v, want nil", got)
	}
}
