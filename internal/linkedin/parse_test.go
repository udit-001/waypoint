package linkedin

import (
	"slices"
	"testing"
)

// fixtureMarkdown is modeled on real Exa web_fetch_exa output for a public
// LinkedIn profile (verified against live fetches of several profiles): the
// title/URL header, name + headline + location block, connections boilerplate,
// an Experience section with both "### Role - [Company]" and nested
// "### [Company]" + "#### Role" forms, an Education section with
// "Degree at [Institution]" and link-only headings, and a bullet-separated
// Skills section.
const fixtureMarkdown = `# Jane Doe
URL: https://www.linkedin.com/in/janedoe
Published: 2026-08-09

# Jane Doe

AI Engineering & DevRel Leader, Director, Google Cloud AI

San Francisco Bay Area (US)

500 connections • 286,740 followers

## About

Jane is an Irish Software Engineer and leader. Total Experience: 25 years and 7 months

## Experience

### Author - [O'Reilly](https://www.linkedin.com/company/oreilly) (Current)

May 2010 - Present (16 years and 3 months)

Author of multiple #1 best-sellers including "Leading Effective Engineering Teams"

Department: Other

### Owner and Author - AddyOsmani.com (Current)

Jan 2005 - Present (21 years and 5 months)

Between my Medium channel and personal site, the canonical technical articles I author have been viewed more than 17M times.

Department: General Management • Level: Owner

### [Google](https://www.linkedin.com/company/google)

#### Director, Google Cloud AI (Current)

Dec 2025 - Present (6 months) in Sunnyvale, California, United States

Gemini! Achieved Cloud AI's annual growth goal in <6 months.

Department: Engineering and Technical • Level: Director

#### Head of Chrome Developer Experience

Jun 2022 - Dec 2025 (3 years and 6 months) in Mountain View, California, United States

Leading Chrome's global engineering teams focused on activating the best of quality web applications.

Google (GOOGL) is a Software Development company. Google is a technology company that offers a wide range of services.

### jQuery team (Bug Triage Team, API Docs Team, Core) - [jQuery Project](https://www.linkedin.com/company/jquery-project)

Aug 2010 - Feb 2013 (2 years and 6 months)

jQuery Application Architecture

jQuery Project is a nonprofit organization. jQuery is a library that provides support and updates for JavaScript.

### Senior Software Engineer (Web Development, JavaScript) - Shortsale Engineering

Mar 2009 - Mar 2011 (2 years)

Department: Engineering and Technical • Level: Senior

## Education

### Masters, Computer Science by Research at [University of Warwick](https://www.linkedin.com/school/uniofwarwick)

2007 - 2009 (2 years) in Coventry, United Kingdom

Awarded for the Thesis: Wavelet-based Geodesic Active Regions

University of Warwick is a higher education institution. University of Warwick is a leading UK university.

### [Harvard University](https://www.linkedin.com/school/harvard-university)

1973 - 1975 (2 years) in Cambridge, Massachusetts, United States

### [Lakeside School](https://www.linkedin.com/school/lakeside-school)

Seattle, Washington, United States

Lakeside School is an educational institution.

## Skills

.net • across • agent development • agent development • agile methodologies • show all 5 skills

## Publications

Total of 95 works and 2,674 citations
`

func TestParseProfile(t *testing.T) {
	p := ParseProfile(fixtureMarkdown)

	if p.Name != "Jane Doe" {
		t.Errorf("Name = %q, want %q", p.Name, "Jane Doe")
	}
	if p.Headline != "AI Engineering & DevRel Leader, Director, Google Cloud AI" {
		t.Errorf("Headline = %q", p.Headline)
	}
	if p.Location != "San Francisco Bay Area (US)" {
		t.Errorf("Location = %q", p.Location)
	}

	// Experience: 6 entries — 4 direct ### roles + 2 nested #### roles.
	if len(p.Exp) != 6 {
		t.Fatalf("len(Exp) = %d, want 6: %+v", len(p.Exp), p.Exp)
	}

	// Direct role with linked company + (Current): dates become partial ISO.
	e := p.Exp[0]
	if e.Title != "Author" || e.Company != "O'Reilly" {
		t.Errorf("Exp[0] = %+v, want {Author O'Reilly}", e)
	}
	if e.Start != "2010-05" || e.End != "" {
		t.Errorf("Exp[0] dates = %q..%q, want 2010-05..(empty present)", e.Start, e.End)
	}
	// Personal description captured; the "Department:" metadata and the
	// company blurb after it are excluded.
	if e.Description != `Author of multiple #1 best-sellers including "Leading Effective Engineering Teams"` {
		t.Errorf("Exp[0] Description = %q", e.Description)
	}

	// Plain company name + (Current) marker on the heading.
	if e2 := p.Exp[1]; e2.Title != "Owner and Author" || e2.Company != "AddyOsmani.com" || e2.Start != "2005-01" {
		t.Errorf("Exp[1] = %+v", e2)
	}

	// Nested company group: #### roles inherit the ### company.
	if e3 := p.Exp[2]; e3.Title != "Director, Google Cloud AI" || e3.Company != "Google" || e3.Start != "2025-12" || e3.End != "" {
		t.Errorf("Exp[2] (nested current) = %+v", e3)
	}
	e4 := p.Exp[3]
	if e4.Title != "Head of Chrome Developer Experience" || e4.Company != "Google" || e4.Start != "2022-06" || e4.End != "2025-12" {
		t.Errorf("Exp[3] (nested past) = %+v", e4)
	}
	// The Google company blurb must not leak into the nested role's description.
	if e4.Description != "Leading Chrome's global engineering teams focused on activating the best of quality web applications." {
		t.Errorf("Exp[3] Description = %q", e4.Description)
	}

	// Company blurb filtered even without a Department: line (jQuery).
	if e5 := p.Exp[4]; e5.Title != "jQuery team (Bug Triage Team, API Docs Team, Core)" || e5.Company != "jQuery Project" {
		t.Errorf("Exp[4] = %+v", e5)
	} else if e5.Description != "jQuery Application Architecture" {
		t.Errorf("Exp[4] Description = %q", e5.Description)
	}

	// Role with no company link and no description.
	if e6 := p.Exp[5]; e6.Title != "Senior Software Engineer (Web Development, JavaScript)" || e6.Company != "Shortsale Engineering" || e6.Start != "2009-03" || e6.End != "2011-03" || e6.Description != "" {
		t.Errorf("Exp[5] = %+v", e6)
	}

	// Education: 3 entries.
	if len(p.Edu) != 3 {
		t.Fatalf("len(Edu) = %d, want 3: %+v", len(p.Edu), p.Edu)
	}
	edu := p.Edu[0]
	if edu.Institution != "University of Warwick" || edu.Degree != "Masters, Computer Science by Research" {
		t.Errorf("Edu[0] = %+v", edu)
	}
	if edu.Start != "2007" || edu.End != "2009" {
		t.Errorf("Edu[0] dates = %q..%q, want 2007..2009", edu.Start, edu.End)
	}
	// Personal description captured; institution blurb filtered.
	if edu.Description != "Awarded for the Thesis: Wavelet-based Geodesic Active Regions" {
		t.Errorf("Edu[0] Description = %q", edu.Description)
	}

	// Link-only institution heading (no degree).
	if edu2 := p.Edu[1]; edu2.Institution != "Harvard University" || edu2.Degree != "" || edu2.Start != "1973" || edu2.End != "1975" {
		t.Errorf("Edu[1] = %+v", edu2)
	}
	// No dates: the location-only line must not become dates or description.
	if edu3 := p.Edu[2]; edu3.Institution != "Lakeside School" || edu3.Start != "" || edu3.End != "" || edu3.Description != "" {
		t.Errorf("Edu[2] = %+v", edu3)
	}

	// Skills: bullet-separated, deduped, noise ("show all 5 skills") dropped.
	want := []string{".net", "across", "agent development", "agile methodologies"}
	if !slices.Equal(p.Skills, want) {
		t.Errorf("Skills = %v, want %v", p.Skills, want)
	}
}

func TestParseProfileEmpty(t *testing.T) {
	p := ParseProfile("No content found for the provided URL(s).")
	if !p.Empty() {
		t.Errorf("Empty() = false, want true for %+v", p)
	}
}

func TestParseProfileTitleFallback(t *testing.T) {
	// LinkedIn-style page title with headline + company baked in — the name
	// parser takes the segment before " - " and strips " | LinkedIn".
	p := ParseProfile("# Jane Doe - Senior Engineer - Acme Corp | LinkedIn\nURL: https://www.linkedin.com/in/janedoe\n\n# Jane Doe\n\nSenior Engineer\n\nLondon, United Kingdom\n")
	if p.Name != "Jane Doe" {
		t.Errorf("Name = %q, want Jane Doe", p.Name)
	}
	if p.Headline != "Senior Engineer" || p.Location != "London, United Kingdom" {
		t.Errorf("Headline/Location = %q/%q", p.Headline, p.Location)
	}
}

func TestDocDropsInvalidEntries(t *testing.T) {
	p := Profile{
		Name:     "  Jane Doe  ",
		Headline: "Engineer",
		Exp: []Experience{
			{Title: "Engineer", Company: "Acme", Start: "2020-01", End: ""},
			{Company: "No Title Here"}, // dropped: title required
		},
		Edu: []Education{
			{Institution: "MIT", Degree: "BS", Start: "2016", End: "2020"},
			{Degree: "No institution"}, // dropped: institution required
		},
		Skills: []string{"Go", "  ", "Rust"},
	}
	doc := p.Doc()

	if doc["name"] != "Jane Doe" {
		t.Errorf("doc[name] = %v", doc["name"])
	}
	exp, _ := doc["experience"].([]map[string]string)
	if len(exp) != 1 || exp[0]["title"] != "Engineer" {
		t.Errorf("doc[experience] = %v, want just the Engineer entry", doc["experience"])
	}
	edu, _ := doc["education"].([]map[string]string)
	if len(edu) != 1 || edu[0]["institution"] != "MIT" {
		t.Errorf("doc[education] = %v, want just the MIT entry", doc["education"])
	}
	skills, _ := doc["skills"].([]string)
	if !slices.Equal(skills, []string{"Go", "Rust"}) {
		t.Errorf("doc[skills] = %v, want [Go Rust]", skills)
	}
	// Empty description key present (PATCH accepts it), but no junk keys.
	if _, ok := doc["email"]; ok {
		t.Error("doc should not contain email (not parsed from LinkedIn)")
	}
}

func TestConvertDate(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"Jan 2000", "2000-01", true},
		{"Dec 2025", "2025-12", true},
		{"September 2020", "2020-09", true},
		{"1973", "1973", true},
		{"Present", "", true},
		{"present", "", true},
		{"", "", false},
		{"next week", "", false},
		{"20000", "", false},
	}
	for _, c := range cases {
		got, ok := convertDate(c.in)
		if ok != c.wantOK || got != c.want {
			t.Errorf("convertDate(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.wantOK)
		}
	}
}
