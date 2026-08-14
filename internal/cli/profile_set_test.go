package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
)

// writeDocFile writes a patch document to a temp file and returns its path.
func writeDocFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write doc file: %v", err)
	}
	return p
}

func setDocFlag(t *testing.T, path string) {
	t.Helper()
	if err := profileSetCmd.Flags().Set("file", path); err != nil {
		t.Fatalf("Set --file: %v", err)
	}
}

// TestProfileSetFromFilePatch: patch semantics — only keys present in the
// document change; everything else stays untouched.
func TestProfileSetFromFilePatch(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	fake.UpsertProfile(map[string]any{"name": "Jane", "remote": "hybrid"})

	setDocFlag(t, writeDocFile(t, `{"remote":"remote"}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	p, _ := fake.GetProfile()
	if p.Remote != "remote" {
		t.Errorf("Remote = %q, want remote", p.Remote)
	}
	if p.Name != "Jane" {
		t.Errorf("Name = %q, want Jane — patch must not touch absent keys", p.Name)
	}
}

// TestProfileSetFromFileScalarsAndLists: scalar strings and list arrays land
// in store-ready form.
func TestProfileSetFromFileScalarsAndLists(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, writeDocFile(t, `{
		"name": "Jane Doe",
		"currentLocation": "Bengaluru",
		"skills": ["Go", "React"],
		"companies": ["Acme", "Globex"]
	}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	p, _ := fake.GetProfile()
	if p.Name != "Jane Doe" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.CurrentLocation != "Bengaluru" {
		t.Errorf("CurrentLocation = %q", p.CurrentLocation)
	}
	if p.Skills != `["Go","React"]` {
		t.Errorf("Skills = %q", p.Skills)
	}
	if p.Companies != `["acme","globex"]` {
		t.Errorf("Companies = %q, want normalized match form (store seam normalizes)", p.Companies)
	}
}

// TestProfileSetFromFileClear: an explicit empty value clears the field
// (set → open in the brief) rather than being ignored.
func TestProfileSetFromFileClear(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	fake.UpsertProfile(map[string]any{"remote": "hybrid", "companies": `["acme"]`})

	setDocFlag(t, writeDocFile(t, `{"remote":"","companies":[]}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	p, _ := fake.GetProfile()
	if p.Remote != "" {
		t.Errorf("Remote = %q, want cleared to ''", p.Remote)
	}
	if p.Companies != "[]" {
		t.Errorf("Companies = %q, want []", p.Companies)
	}

	// Clearing flips it back to open in the brief.
	b, _ := fake.GetBrief()
	for _, o := range b.Open {
		if o == "remote" {
			return
		}
	}
	t.Errorf("expected 'remote' back in brief open after clear, got %v", b.Open)
}

// TestProfileSetFromFileSalaryFloor: the [{region, amount}] doc form; region
// is required (currency is derived, never accepted).
func TestProfileSetFromFileSalaryFloor(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, writeDocFile(t, `{"salaryFloor":[{"region":"IN","amount":100000},{"region":"GB","amount":30000}]}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	p, _ := fake.GetProfile()
	if p.SalaryFloor != `[{"region":"IN","amount":100000},{"region":"GB","amount":30000}]` {
		t.Errorf("SalaryFloor = %q", p.SalaryFloor)
	}

	// Missing region is a hard error, not a silent drop.
	setDocFlag(t, writeDocFile(t, `{"salaryFloor":[{"amount":100000}]}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err == nil {
		t.Fatal("expected error for salary floor without region, got nil")
	}
}

// TestProfileSetFromFileExperienceEducation: structured entries (with
// description) round-trip through the doc.
func TestProfileSetFromFileExperienceEducation(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, writeDocFile(t, `{
		"experience": [{"title":"Senior SWE","company":"Acme","start":"2021-03","end":"2023-06","description":"Led payments team of 5"}],
		"education": [{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019","description":"GPA 3.9"}]
	}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	p, _ := fake.GetProfile()
	wantExp := `[{"title":"Senior SWE","company":"Acme","start":"2021-03","end":"2023-06","description":"Led payments team of 5"}]`
	if p.Experience != wantExp {
		t.Errorf("Experience = %q, want %q", p.Experience, wantExp)
	}
	wantEdu := `[{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019","description":"GPA 3.9"}]`
	if p.Education != wantEdu {
		t.Errorf("Education = %q, want %q", p.Education, wantEdu)
	}
}

// TestProfileSetFromFileValidation: entry rule errors surface with the doc key.
func TestProfileSetFromFileValidation(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, writeDocFile(t, `{"experience":[{"company":"Acme"}]}`))
	err := profileSetCmd.RunE(profileSetCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "experience: entry 1: title is required") {
		t.Fatalf("error = %v, want experience: entry 1: title is required", err)
	}

	setDocFlag(t, writeDocFile(t, `{"experience":[{"title":"SWE","start":"03/2021"}]}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err == nil {
		t.Fatal("expected error for invalid start date, got nil")
	}
}

// TestProfileSetFromFileSeniorityGate: manual seniority is rejected once
// experience carries a derived level.
func TestProfileSetFromFileSeniorityGate(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	// No experience → manual seniority is the placeholder.
	setDocFlag(t, writeDocFile(t, `{"seniority":"mid"}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE (no experience): %v", err)
	}
	if p, _ := fake.GetProfile(); p.Seniority != "mid" {
		t.Errorf("expected seniority stored when no experience, got %q", p.Seniority)
	}

	// Experience with a year signal → manual seniority is rejected, stored
	// value unchanged.
	fake.UpsertProfile(map[string]any{"experience": `["8 years in genomics"]`})
	setDocFlag(t, writeDocFile(t, `{"seniority":"junior"}`))
	err := profileSetCmd.RunE(profileSetCmd, nil)
	if err == nil {
		t.Fatal("expected error when experience derives seniority, got nil")
	}
	if p, _ := fake.GetProfile(); p.Seniority != "mid" {
		t.Errorf("stored seniority should be unchanged, got %q", p.Seniority)
	}
}

// TestProfileSetFromFileUnknownKey: a typo is a hard error, never a silent no-op.
func TestProfileSetFromFileUnknownKey(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, writeDocFile(t, `{"namee":"Jane"}`))
	err := profileSetCmd.RunE(profileSetCmd, nil)
	if err == nil || !strings.Contains(err.Error(), `unknown profile field "namee"`) {
		t.Fatalf("error = %v, want unknown profile field", err)
	}
}

// TestProfileSetFromFileEmptyAndMalformed: empty/not-an-object docs are errors.
func TestProfileSetFromFileEmptyAndMalformed(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, writeDocFile(t, `{}`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err == nil {
		t.Fatal("expected error for empty doc, got nil")
	}

	setDocFlag(t, writeDocFile(t, `[{"name":"Jane"}]`))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err == nil {
		t.Fatal("expected error for non-object doc, got nil")
	}
}

// TestProfileSetFromFileMissing: a missing file is an error, not a silent no-op.
func TestProfileSetFromFileMissing(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	setDocFlag(t, filepath.Join(t.TempDir(), "missing.json"))
	if err := profileSetCmd.RunE(profileSetCmd, nil); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestProfileSetFromStdin: --file - reads the document from stdin.
func TestProfileSetFromStdin(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = false

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString(`{"name":"From Stdin"}`); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	setDocFlag(t, "-")
	if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	p, _ := fake.GetProfile()
	if p.Name != "From Stdin" {
		t.Errorf("Name = %q, want From Stdin", p.Name)
	}
}

// TestProfileSetJSONOutput: --json prints the updated profile.
func TestProfileSetJSONOutput(t *testing.T) {
	fake := db.NewFakeStore()
	store = fake
	jsonOut = true

	setDocFlag(t, writeDocFile(t, `{"name":"Jane"}`))
	out := captureStdout(t, func() {
		if err := profileSetCmd.RunE(profileSetCmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
	})
	if !strings.Contains(out, `"name": "Jane"`) {
		t.Errorf("JSON output missing name field: %q", out)
	}
}

// TestProfileSchema: the schema command prints the empty writable template.
func TestProfileSchema(t *testing.T) {
	out := captureStdout(t, func() {
		if err := profileSchemaCmd.RunE(profileSchemaCmd, nil); err != nil {
			t.Fatalf("RunE: %v", err)
		}
	})

	for _, key := range []string{
		`"name"`, `"currentLocation"`, `"skills"`, `"salaryFloor"`,
		`"experience"`, `"education"`, `"description"`, `"title"`,
	} {
		if !strings.Contains(out, key) {
			t.Errorf("schema output missing %s: %q", key, out)
		}
	}
	// The template is a JSON object (the fill-in-the-blank doc).
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("schema output is not a JSON object: %q", out)
	}
}
