package linkedin

import (
	"strings"
	"testing"
)

// Imported experience lines carry their own "- " bullets; storing them
// verbatim double-marks every line at render time (render adds "•").
func TestJoinDesc_stripsBulletMarkers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"- Team Engagement: Led initiatives", "Team Engagement: Led initiatives"},
		{"• Client Training: PPOC for clients", "Client Training: PPOC for clients"},
		{"* Project coordination: Rolling out", "Project coordination: Rolling out"},
		{"Plain sentence without marker", "Plain sentence without marker"},
	}
	for _, c := range cases {
		if got := joinDesc("", c.in); got != c.want {
			t.Errorf("joinDesc(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// Multi-line accumulation stays clean across joins.
	got := joinDesc(joinDesc("", "- First line"), "Second line")
	want := "First line\nSecond line"
	if got != want {
		t.Errorf("joined = %q, want %q", got, want)
	}
}

// Some public profiles render their skill list INSIDE an education entry
// ("Skills: A · B · C") and have no ## Skills section at all (reported
// against a real import). Those lines must populate Skills,
// not glue onto the education description.
func TestParse_skillsLineInsideEducation(t *testing.T) {
	md := `# Test Candidate

## Education

### Master of Business Administration - MBA at State University

2021 - 2023 (2 years) in New Delhi, Delhi, India

Skills: R (Programming Language) · Microsoft Power BI · SQL · Strategy

Department of Business Economics is an educational institution.
`
	p := ParseProfile(md)

	if len(p.Edu) != 1 {
		t.Fatalf("edu entries = %d, want 1", len(p.Edu))
	}
	// The skills line must not land in the education description.
	// (The company blurb line below it is a separate, pre-existing
	// parser concern.)
	if strings.Contains(p.Edu[0].Description, "Skills:") || strings.Contains(p.Edu[0].Description, "Power BI") {
		t.Errorf("education description = %q, still carries the skills list", p.Edu[0].Description)
	}
	want := []string{"R (Programming Language)", "Microsoft Power BI", "SQL", "Strategy"}
	if len(p.Skills) != len(want) {
		t.Fatalf("skills = %v, want %v", p.Skills, want)
	}
	for i, w := range want {
		if p.Skills[i] != w {
			t.Errorf("skills[%d] = %q, want %q", i, p.Skills[i], w)
		}
	}
}

// Education-only profile with no Skills section in the source at all:
// parse must not crash or invent entries.
func TestParse_educationOnlyProfile(t *testing.T) {
	md := `# Test User

M.Sc. Biotechnology 2nd year | Illustrator

## Education

### M.Sc. Biotechnology at State Institute

2023 - Present in India

State Institute of Engineering & Technology is a higher education institution.

### Bachelor of Science - BS at City College

2021 - 2023 (2 years) in Delhi
`
	p := ParseProfile(md)
	if len(p.Edu) != 2 {
		t.Errorf("edu = %d, want 2", len(p.Edu))
	}
	if len(p.Skills) != 0 {
		t.Errorf("skills = %v, want empty (source has none)", p.Skills)
	}
	for _, e := range p.Edu {
		if strings.Contains(e.Description, "Skills:") {
			t.Errorf("edu @%q description carries a skills line", e.Institution)
		}
	}
}
