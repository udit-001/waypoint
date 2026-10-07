package db_test

import (
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/db/dbtest"
)

// TestBriefText_Allowlist pins the PII seam (spec WP-DOC-8, hard constraint
// "PII never reaches an LLM"). Brief.LLMInput().Text() is the single renderer
// for every LLM input in the system, so this one test covers the allowlist
// for every builder; the two builders are additionally asserted at the wire.
//
// Default-deny is the rule, so the absence half is the interesting one. If a
// future change wants to render something here, this test is the conversation.
func TestBriefText_Allowlist(t *testing.T) {
	f := db.NewFakeStore()
	f.Profile = dbtest.PIIProfile()

	brief, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}
	// The caller's path: profile -> brief -> allowlist projection -> text.
	out := brief.LLMInput().Text()

	for _, c := range dbtest.NeverLeaves() {
		if strings.Contains(out, c.Value) {
			t.Errorf("%s leaked into the LLM input: %q found in:\n%s", c.What, c.Value, out)
		}
	}

	mustAppear := []dbtest.Leak{
		{What: "title", Value: "Backend Engineer"},
		{What: "seniority", Value: "senior"},
		{What: "industry", Value: "fintech"},
		{What: "skill", Value: "PostgreSQL"},
		{What: "interest", Value: "distributed systems"},
		{What: "company liked", Value: "Gojek"},
		{What: "company avoided", Value: "Acme"},
		{What: "remote preference", Value: "remote"},
		{What: "city-granularity location preference", Value: "Berlin"},
		{What: "past role", Value: "Senior Engineer"},
		{What: "past company", Value: "Stripe"},
	}
	for _, c := range mustAppear {
		if !strings.Contains(out, c.Value) {
			t.Errorf("%s missing from the LLM input: %q not found in:\n%s", c.What, c.Value, out)
		}
	}
}

// TestLLMInput_SkillsAreCurated: the renderer dedupes skills and drops
// generic filler tags — a resume-sourced profile carries several, and they
// crowd out the real signal.
func TestLLMInput_SkillsAreCurated(t *testing.T) {
	f := db.NewFakeStore()
	f.Profile = db.Profile{
		Skills: `["Go","go","  Go  ","Engineer","software engineer","PostgreSQL"]`,
	}
	brief, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}

	got := brief.LLMInput().Skills
	want := []string{"Go", "PostgreSQL"}
	if len(got) != len(want) {
		t.Fatalf("skills = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("skills[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestCompressWorkHistory: the history the model sees is role + company, in
// profile order, deduped and capped — and never the free-text description.
func TestCompressWorkHistory(t *testing.T) {
	entries := make([]string, 0, 14)
	for i := 0; i < 14; i++ {
		entries = append(entries, `{"title":"Engineer `+string(rune('A'+i))+`","company":"Co `+string(rune('A'+i))+`"}`)
	}
	// A duplicate of the first entry, and a blank one, both dropped.
	stored := "[" + strings.Join(entries, ",") + `,{"title":"Engineer A","company":"Co A"},{"title":"","company":""}]`

	f := db.NewFakeStore()
	f.Profile = db.Profile{Experience: stored}
	brief, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}

	got := brief.LLMInput().WorkHistory
	if len(got) != 10 {
		t.Fatalf("work history = %d entries, want the cap of 10: %+v", len(got), got)
	}
	if got[0].Title != "Engineer A" || got[0].Company != "Co A" {
		t.Errorf("first entry = %+v, want Engineer A @ Co A (profile order)", got[0])
	}
	if got[1].Title != "Engineer B" {
		t.Errorf("second entry = %+v, want Engineer B — the duplicate must collapse", got[1])
	}
	if strings.Contains(brief.LLMInput().Text(), "description") {
		t.Error("free-text description reached the LLM input")
	}
}
