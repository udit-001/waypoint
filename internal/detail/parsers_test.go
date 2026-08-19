package detail

import (
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// --- Greenhouse fixture ---

const ghFixture = `# Senior Software Engineer

at SpaceX

Location: Hawthorne, CA

## About the Role

We are looking for a Senior Software Engineer to join our team.

## Requirements

* 5+ years of experience in Go or Rust
* Experience with distributed systems
* Strong understanding of networking protocols

## Benefits

* Health, dental, and vision insurance
* 401(k) matching

## Compensation

Pay range: $150,000 – $190,000 a year

Apply by: September 15, 2026
`

func TestGreenhouseParser_TitleCompany(t *testing.T) {
	p := ParserFor("greenhouse")
	if p == nil {
		t.Fatal("greenhouse parser not registered")
	}
	r := p.Parse(ghFixture)
	if r.Title != "Senior Software Engineer" {
		t.Errorf("title = %q, want %q", r.Title, "Senior Software Engineer")
	}
	if r.Company != "SpaceX" {
		t.Errorf("company = %q, want %q", r.Company, "SpaceX")
	}
}

func TestGreenhouseParser_Salary(t *testing.T) {
	p := ParserFor("greenhouse")
	r := p.Parse(ghFixture)
	if r.Salary != "$150,000 – $190,000 a year" {
		t.Errorf("salary = %q, want %q", r.Salary, "$150,000 – $190,000 a year")
	}
}

func TestGreenhouseParser_Deadline(t *testing.T) {
	p := ParserFor("greenhouse")
	r := p.Parse(ghFixture)
	if r.Deadline != "September 15, 2026" {
		t.Errorf("deadline = %q, want %q", r.Deadline, "September 15, 2026")
	}
}

func TestGreenhouseParser_Requirements(t *testing.T) {
	p := ParserFor("greenhouse")
	r := p.Parse(ghFixture)
	reqs := strings.Split(r.Requirements, "\n")
	if len(reqs) != 3 {
		t.Fatalf("requirements = %d lines, want 3: %q", len(reqs), r.Requirements)
	}
	if reqs[0] != "5+ years of experience in Go or Rust" {
		t.Errorf("reqs[0] = %q", reqs[0])
	}
}

func TestGreenhouseParser_BoldQualifications(t *testing.T) {
	// Bug fix test: accept **QUALIFICATIONS**-style headers.
	md := `# Data Scientist

at Acme Corp

## **QUALIFICATIONS**

* PhD in Computer Science
* 3+ years of experience

Pay range: $120,000 – $150,000
`
	p := ParserFor("greenhouse")
	r := p.Parse(md)
	reqs := strings.Split(r.Requirements, "\n")
	if len(reqs) != 2 {
		t.Errorf("bold reqs = %d lines, want 2: %q", len(reqs), r.Requirements)
	}
}

// Parsers no longer stamp a source — the chain knows which tier ran
// and stamps provenance (parseBody). A parser result must carry none.
func TestGreenhouseParser_NoSourceStamp(t *testing.T) {
	p := ParserFor("greenhouse")
	r := p.Parse(ghFixture)
	if r.Source != "" {
		t.Errorf("source = %q, want empty (chain stamps provenance)", r.Source)
	}
}

// --- Lever fixture ---

const leverFixture = `MobileAction - Senior Backend Engineer

We are building the next generation of mobile analytics.

## What you'll bring

* 5+ years backend experience
* Go, Python, or Java proficiency
* Experience with data pipelines

Compensation: $130,000 – $170,000 per year

Application deadline: October 1, 2026

Remote / Hybrid
`

func TestLeverParser_TitleCompany(t *testing.T) {
	p := ParserFor("lever")
	if p == nil {
		t.Fatal("lever parser not registered")
	}
	r := p.Parse(leverFixture)
	if r.Title != "Senior Backend Engineer" {
		t.Errorf("title = %q, want %q", r.Title, "Senior Backend Engineer")
	}
	if r.Company != "MobileAction" {
		t.Errorf("company = %q, want %q", r.Company, "MobileAction")
	}
}

func TestLeverParser_Salary(t *testing.T) {
	p := ParserFor("lever")
	r := p.Parse(leverFixture)
	if r.Salary != "$130,000 – $170,000 per year" {
		t.Errorf("salary = %q, want %q", r.Salary, "$130,000 – $170,000 per year")
	}
}

func TestLeverParser_Deadline(t *testing.T) {
	p := ParserFor("lever")
	r := p.Parse(leverFixture)
	if r.Deadline != "October 1, 2026" {
		t.Errorf("deadline = %q, want %q", r.Deadline, "October 1, 2026")
	}
}

func TestLeverParser_Location(t *testing.T) {
	p := ParserFor("lever")
	r := p.Parse(leverFixture)
	if r.Location != "Remote" {
		t.Errorf("location = %q, want %q", r.Location, "Remote")
	}
}

func TestLeverParser_Requirements(t *testing.T) {
	p := ParserFor("lever")
	r := p.Parse(leverFixture)
	reqs := strings.Split(r.Requirements, "\n")
	if len(reqs) != 3 {
		t.Fatalf("requirements = %d lines, want 3: %q", len(reqs), r.Requirements)
	}
}

// --- Ashby fixture ---

const ashbyFixture = `Software Engineer @ Circleback

We're building the future of note-taking.

## What You'll Do

* Build and ship features end-to-end
* Work closely with design and product
* Improve our AI-powered transcription

## Qualifications

* 3+ years software engineering experience
* Strong TypeScript and React skills
* Experience with real-time systems

₹15–22 LPA

Bangalore, India

Apply by: August 30, 2026
`

func TestAshbyParser_TitleCompany(t *testing.T) {
	p := ParserFor("ashby")
	if p == nil {
		t.Fatal("ashby parser not registered")
	}
	r := p.Parse(ashbyFixture)
	if r.Title != "Software Engineer" {
		t.Errorf("title = %q, want %q", r.Title, "Software Engineer")
	}
	if r.Company != "Circleback" {
		t.Errorf("company = %q, want %q", r.Company, "Circleback")
	}
}

func TestAshbyParser_Salary(t *testing.T) {
	p := ParserFor("ashby")
	r := p.Parse(ashbyFixture)
	if r.Salary != "₹15–22 LPA" {
		t.Errorf("salary = %q, want %q", r.Salary, "₹15–22 LPA")
	}
}

func TestAshbyParser_Deadline(t *testing.T) {
	p := ParserFor("ashby")
	r := p.Parse(ashbyFixture)
	if r.Deadline != "August 30, 2026" {
		t.Errorf("deadline = %q, want %q", r.Deadline, "August 30, 2026")
	}
}

func TestAshbyParser_Location(t *testing.T) {
	p := ParserFor("ashby")
	r := p.Parse(ashbyFixture)
	if r.Location != "Bangalore, India" {
		t.Errorf("location = %q, want %q", r.Location, "Bangalore, India")
	}
}

func TestAshbyParser_Requirements(t *testing.T) {
	p := ParserFor("ashby")
	r := p.Parse(ashbyFixture)
	reqs := strings.Split(r.Requirements, "\n")
	if len(reqs) != 3 {
		t.Fatalf("requirements = %d lines, want 3: %q", len(reqs), r.Requirements)
	}
}

// --- Generic fixture (no family) ---

const genericFixture = `# Research Scientist

Acme Labs is hiring a Research Scientist.

## Requirements

* PhD in biology or related field
* 2+ years postdoc experience

Salary: $100,000 – $130,000

Remote
`

func TestGenericParser_NoTitleExtraction(t *testing.T) {
	// No parser is registered for "generic", so no structured fields
	// are extracted. The body is still available.
	family := DetectFamily("https://example.com/careers/123")
	if family != "" {
		t.Errorf("family = %q, want empty", family)
	}
}

// --- DetectFamily tests ---

func TestDetectFamily_Greenhouse(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://boards.greenhouse.io/spacex/jobs/7690206", "greenhouse"},
		{"https://boards.eu.greenhouse.io/test/jobs/123", "greenhouse"},
		{"https://job-boards.greenhouse.io/test/jobs/456", "greenhouse"},
	}
	for _, tt := range tests {
		if got := DetectFamily(tt.url); got != tt.want {
			t.Errorf("DetectFamily(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDetectFamily_Lever(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://jobs.lever.co/company/abc123", "lever"},
		{"https://jobs.eu.lever.co/company/abc123", "lever"},
	}
	for _, tt := range tests {
		if got := DetectFamily(tt.url); got != tt.want {
			t.Errorf("DetectFamily(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDetectFamily_Ashby(t *testing.T) {
	if got := DetectFamily("https://jobs.ashbyhq.com/company/abc123"); got != "ashby" {
		t.Errorf("DetectFamily = %q, want %q", got, "ashby")
	}
}

func TestDetectFamily_LinkedIn(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.linkedin.com/jobs/view/12345", "linkedin"},
		{"https://in.linkedin.com/jobs/view/12345", "linkedin"},
	}
	for _, tt := range tests {
		if got := DetectFamily(tt.url); got != tt.want {
			t.Errorf("DetectFamily(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestDetectFamily_Unknown(t *testing.T) {
	if got := DetectFamily("https://example.com/careers/123"); got != "" {
		t.Errorf("DetectFamily = %q, want empty", got)
	}
}

// --- Merge tests ---

func TestMerge_SweepWinsURLCompanyTitle(t *testing.T) {
	listing := scraperResult("url1", "Listing Title", "Listing Co", "2026-01-01", "Old, TX")
	detail := DetailResult{
		Source:   "exa",
		Title:    "Detail Title",
		Company:  "Detail Co",
		Location: "New, NY",
		Salary:   "$100k",
		Body:     "full text",
	}
	merged := Merge(listing, detail, "old desc", nil)

	if merged.URL != "url1" {
		t.Errorf("URL = %q, want url1", merged.URL)
	}
	if merged.Title != "Listing Title" {
		t.Errorf("Title = %q, want Listing Title", merged.Title)
	}
	if merged.Company != "Listing Co" {
		t.Errorf("Company = %q, want Listing Co", merged.Company)
	}
	if merged.Location != "New, NY" {
		t.Errorf("Location = %q, want New, NY", merged.Location)
	}
}

func TestMerge_DetailWinsDescriptionSalary(t *testing.T) {
	listing := scraperResult("url1", "Title", "Co", "2026-01-01", "")
	detail := DetailResult{
		Source:      "board",
		Description: "Full description from board",
		Salary:      "$150k",
		Deadline:    "2026-09-01",
	}
	merged := Merge(listing, detail, "listing desc", nil)

	if merged.Description != "Full description from board" {
		t.Errorf("Description = %q", merged.Description)
	}
	if merged.Metadata["salary"] != "$150k" {
		t.Errorf("metadata[salary] = %q", merged.Metadata["salary"])
	}
	if merged.Metadata["deadline"] != "2026-09-01" {
		t.Errorf("metadata[deadline] = %q", merged.Metadata["deadline"])
	}
}

func TestMerge_ListingDescPreservedWhenDetailEmpty(t *testing.T) {
	listing := scraperResult("url1", "Title", "Co", "2026-01-01", "")
	detail := DetailResult{Source: "none"}
	merged := Merge(listing, detail, "listing description", nil)

	if merged.Description != "listing description" {
		t.Errorf("Description = %q, want listing description", merged.Description)
	}
}

func TestMerge_MetadataUnion(t *testing.T) {
	listingMeta := map[string]string{"existing": "value", "department": "eng"}
	listing := scraperResult("url1", "Title", "Co", "2026-01-01", "")
	detail := DetailResult{
		Source:   "exa",
		Salary:   "$120k",
		Metadata: map[string]string{"department": "product", "team": "backend"},
	}
	merged := Merge(listing, detail, "", listingMeta)

	if merged.Metadata["existing"] != "value" {
		t.Errorf("existing metadata lost")
	}
	if merged.Metadata["department"] != "product" {
		t.Errorf("department = %q, want product (detail wins)", merged.Metadata["department"])
	}
	if merged.Metadata["team"] != "backend" {
		t.Errorf("team = %q, want backend", merged.Metadata["team"])
	}
	if merged.Metadata["salary"] != "$120k" {
		t.Errorf("salary = %q", merged.Metadata["salary"])
	}
}

func TestMerge_ProvenanceRecorded(t *testing.T) {
	listing := scraperResult("url1", "Title", "Co", "2026-01-01", "")
	detail := DetailResult{Source: "scraper"}
	merged := Merge(listing, detail, "", nil)

	if merged.Metadata["detail_source"] != "scraper" {
		t.Errorf("detail_source = %q, want scraper", merged.Metadata["detail_source"])
	}
}

// --- BodyFieldsFilled tests ---

func TestBodyFieldsFilled_AllPresent(t *testing.T) {
	r := scraperResult("u", "t", "c", "d", "")
	r.Description = "desc"
	r.Metadata = map[string]string{"salary": "$100k", "deadline": "2026-01-01"}
	if !BodyFieldsFilled(r) {
		t.Error("expected true when all fields present")
	}
}

func TestBodyFieldsFilled_MissingDescription(t *testing.T) {
	r := scraperResult("u", "t", "c", "d", "")
	r.Metadata = map[string]string{"salary": "$100k", "deadline": "2026-01-01"}
	if BodyFieldsFilled(r) {
		t.Error("expected false when description empty")
	}
}

func TestBodyFieldsFilled_MissingSalary(t *testing.T) {
	r := scraperResult("u", "t", "c", "d", "")
	r.Description = "desc"
	r.Metadata = map[string]string{"deadline": "2026-01-01"}
	if BodyFieldsFilled(r) {
		t.Error("expected false when salary empty")
	}
}

func TestBodyFieldsFilled_NoMetadata(t *testing.T) {
	r := scraperResult("u", "t", "c", "d", "")
	r.Description = "desc"
	if BodyFieldsFilled(r) {
		t.Error("expected false when no metadata")
	}
}

// --- helpers ---

func scraperResult(id, title, company, date, location string) scraper.Result {
	return scraper.Result{
		ID:       id,
		Title:    title,
		Company:  company,
		Date:     date,
		Location: location,
		URL:      id, // ID doubles as URL in tests
	}
}
