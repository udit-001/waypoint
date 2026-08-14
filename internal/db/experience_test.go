package db

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseExperienceEntries(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		want   []ExperienceEntry
	}{
		{"empty", "", nil},
		{"empty array", "[]", nil},
		{"structured objects pass through", `[{"title":"SWE","company":"Acme","start":"2021-03","end":"2023-06"}]`, []ExperienceEntry{{Title: "SWE", Company: "Acme", Start: "2021-03", End: "2023-06"}}},
		{"legacy flat strings upgrade", `["5 years backend development","Intern"]`, []ExperienceEntry{{Title: "5 years backend development"}, {Title: "Intern"}}},
		{"mixed objects and legacy strings", `["Intern",{"title":"SWE","start":"2021-03"}]`, []ExperienceEntry{{Title: "Intern"}, {Title: "SWE", Start: "2021-03"}}},
		{"blank legacy strings dropped", `["  ",{"title":"SWE"}]`, []ExperienceEntry{{Title: "SWE"}}},
		{"malformed", `not-json`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseExperienceEntries(tc.stored)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseExperienceEntries(%q) = %#v, want %#v", tc.stored, got, tc.want)
			}
		})
	}
}

func TestParseEducationEntries(t *testing.T) {
	got := ParseEducationEntries(`["BS CS - MIT",{"institution":"MIT","degree":"BS","start":"2015","end":"2019"}]`)
	want := []EducationEntry{
		{Institution: "BS CS - MIT"},
		{Institution: "MIT", Degree: "BS", Start: "2015", End: "2019"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestExperienceToJSONRoundTrip(t *testing.T) {
	entries := []ExperienceEntry{{Title: "SWE", Company: "Acme", Start: "2021-03", End: ""}}
	json, err := experienceToJSON(entries)
	if err != nil {
		t.Fatalf("experienceToJSON: %v", err)
	}
	got := ParseExperienceEntries(json)
	if !reflect.DeepEqual(got, entries) {
		t.Errorf("round-trip = %#v, want %#v", got, entries)
	}
}

func TestPartialDateMonths(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"2021-03", 2021*12 + 2},
		{"2021-01", 2021 * 12},
		{"2021", 2021 * 12},
		{"2019-12", 2019*12 + 11},
		{" 2021-03 ", 2021*12 + 2},
		{"", -1},
		{"2021-13", -1},
		{"2021-00", -1},
		{"garbage", -1},
		{"21-03", -1},
	}
	for _, tc := range tests {
		if got := partialDateMonths(tc.in); got != tc.want {
			t.Errorf("partialDateMonths(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestEntryMonths(t *testing.T) {
	if got := entryMonths("2021-03", "2023-06"); got != 27 {
		t.Errorf("2021-03..2023-06 = %d months, want 27", got)
	}
	if got := entryMonths("2021", "2023"); got != 24 {
		t.Errorf("2021..2023 = %d months, want 24", got)
	}
	// Empty end means "present" — always at least through the current month.
	if got := entryMonths("2021-01", ""); got < 12 {
		t.Errorf("2021-01..present = %d months, want >= 12", got)
	}
	// Reversed dates contribute nothing.
	if got := entryMonths("2023-06", "2021-03"); got != 0 {
		t.Errorf("reversed = %d, want 0", got)
	}
	// No start date → nothing.
	if got := entryMonths("", "2023-06"); got != 0 {
		t.Errorf("no start = %d, want 0", got)
	}
}

func TestDeriveSeniorityStructured(t *testing.T) {
	entries := func(title, start, end string) string {
		json, _ := experienceToJSON([]ExperienceEntry{{Title: title, Company: "Acme", Start: start, End: end}})
		return json
	}
	tests := []struct {
		name       string
		experience string
		want       string
	}{
		{"junior by date range", entries("SWE", "2023-01", "2024-06"), "junior"},
		{"mid by date range", entries("SWE", "2019-01", "2023-06"), "mid"},
		{"senior by date range", entries("SWE", "2016-01", "2024-12"), "senior"},
		{"present counts to now", entries("SWE", "2018-01", ""), "senior"},
		{"no dates and no year signal", entries("SWE", "", ""), ""},
		{"no dates, regex fallback in title", entries("5 years backend", "", ""), "mid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeriveSeniority(tc.experience); got != tc.want {
				t.Errorf("DeriveSeniority(%q) = %q, want %q", tc.experience, got, tc.want)
			}
		})
	}
}

func TestDeriveSeniorityLegacyFallback(t *testing.T) {
	// Flat legacy arrays and free-text year signals still derive.
	if got := DeriveSeniority(`["5 years backend development"]`); got != "mid" {
		t.Errorf("legacy 5 years = %q, want mid", got)
	}
	if got := DeriveSeniority(`["10 years distributed systems"]`); got != "senior" {
		t.Errorf("legacy 10 years = %q, want senior", got)
	}
	if got := DeriveSeniority(""); got != "" {
		t.Errorf("empty = %q, want empty", got)
	}
	if got := DeriveSeniority(`[]`); got != "" {
		t.Errorf("empty array = %q, want empty", got)
	}
}

func TestSerializeExperience(t *testing.T) {
	raw := []byte(`[{"title":"Senior SWE","company":"Acme","start":"2021-03","end":"2023-06"}]`)
	got, err := SerializeExperience(raw)
	if err != nil {
		t.Fatalf("SerializeExperience: %v", err)
	}
	want := `[{"title":"Senior SWE","company":"Acme","start":"2021-03","end":"2023-06"}]`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, tc := range []struct {
		name, raw, wantErr string
	}{
		{"missing title", `[{"company":"Acme"}]`, "title is required"},
		{"bad start date", `[{"title":"SWE","start":"03/2021"}]`, "invalid date"},
		{"bad end date", `[{"title":"SWE","end":"2021-13"}]`, "invalid date"},
		{"not an array", `{"title":"SWE"}`, "must be a JSON array"},
		{"malformed", `garbage`, "must be a JSON array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SerializeExperience([]byte(tc.raw)); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("SerializeExperience(%s) error = %v, want containing %q", tc.raw, err, tc.wantErr)
			}
		})
	}
}

func TestSerializeEducation(t *testing.T) {
	got, err := SerializeEducation([]byte(`[{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019"}]`))
	if err != nil {
		t.Fatalf("SerializeEducation: %v", err)
	}
	want := `[{"institution":"MIT","degree":"BS CS","start":"2015","end":"2019"}]`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if _, err := SerializeEducation([]byte(`[{"degree":"BS"}]`)); err == nil {
		t.Error("expected error for missing institution, got nil")
	}
}

func TestValidatePartialDate(t *testing.T) {
	for _, ok := range []string{"", "2021-03", "2019", "2021-12"} {
		if err := validatePartialDate(ok); err != nil {
			t.Errorf("validatePartialDate(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"2021-13", "garbage", "03/2021", "2021-3"} {
		if err := validatePartialDate(bad); err == nil {
			t.Errorf("validatePartialDate(%q) = nil, want error", bad)
		}
	}
}
