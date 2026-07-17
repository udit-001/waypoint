package ipu

import (
	"context"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<html><body>
<table>
<tbody>
<tr>
<td><a href="/Pubinfo2026/nt090720260219.pdf">Advertisement for Guest Faculty for M.Sc. Courses (CEPS)</a></td>
<td>09-07-2026</td>
</tr>
<tr>
<td><a href="/Pubinfo2026/nt080720260535.pdf">Extension notice for the post of Assistant Professor (On Contract basis)</a></td>
<td>08-07-2026</td>
</tr>
<tr><td colspan="3">June 2026</td></tr>
</tbody>
</table>
</body></html>`

func TestParseTable(t *testing.T) {
	results := parseTable(testHTML)
	if len(results) != 2 {
		t.Fatalf("expected 2 (header row filtered), got %d", len(results))
	}
	r := results[0]
	if r.Title != "Advertisement for Guest Faculty for M.Sc. Courses (CEPS)" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "09-07-2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.ipu.ac.in/Pubinfo2026/nt090720260219.pdf" {
		t.Errorf("URL: got %q", r.URL)
	}
}

func TestParseTable_empty(t *testing.T) {
	if len(parseTable("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := IPU{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "faculty"})
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "faculty"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestFilterNonAds_keepsAdsAndRecordsType(t *testing.T) {
	results := []scraper.Result{
		{Title: "Advertisement for Guest Faculty for M.Sc. Courses (CEPS)"},
		{Title: "Extension notice for the post of Assistant Professor (On Contract basis)"},
		{Title: "Corrigendum - Advertisement for the post of Assistant Professor"},
		{Title: "Walk-in interview for the post of JRF"},
	}
	got := filterNonAds(results)
	if len(got) != 4 {
		t.Fatalf("got %d results, want 4 (all ads kept)", len(got))
	}
	for _, r := range got {
		if r.Metadata["notice_type"] == "" {
			t.Errorf("result %q: notice_type not set", r.Title)
		}
	}
}

func TestFilterNonAds_dropsScheduleVariants(t *testing.T) {
	schedules := []string{
		"Schedule of Interview for the post of Assistant Professor",
		"Interview schedule of Guest Faculty",
		"Revised Schedule of Skill Test",
		"Schedule of Skill Test / Documents Verification",
		"Documents Verification Schedule",
		"Reschedule of Interview",
	}
	results := make([]scraper.Result, 0, len(schedules))
	for _, s := range schedules {
		results = append(results, scraper.Result{Title: s})
	}
	got := filterNonAds(results)
	if len(got) != 0 {
		t.Errorf("expected 0 (all schedules dropped), got %d: %+v", len(got), got)
	}
}

func TestFilterNonAds_dropsOtherNonAdTypes(t *testing.T) {
	cases := []string{
		"Result of Interview for the post of Assistant Professor",
		"List of Selected Candidates for Guest Faculty",
		"Cancellation of Advertisement for the post of JRF",
		"Postponement of Interview Schedule",
		"Refund of Application Fee",
		"Empanelment of Guest Faculty",
		"Empanel - List of Faculty Members",
		"Procurement Notice for Lab Equipment",
		"NIT for Supply of Computers",
		"Notice Inviting Bid for CCTV Installation",
		"Syllabus for M.Sc. Computer Science",
		"Inviting Objections to the Provisional Answer Key",
	}
	results := make([]scraper.Result, 0, len(cases))
	for _, c := range cases {
		results = append(results, scraper.Result{Title: c})
	}
	got := filterNonAds(results)
	if len(got) != 0 {
		t.Errorf("expected 0 (all non-ads dropped), got %d: %+v", len(got), got)
	}
}

func TestFilterNonAds_keepsUnmatchedConservative(t *testing.T) {
	// Titles that match no blacklist pattern must be kept (better to include
	// than miss a real ad).
	unmatched := []string{
		"Notification regarding Guest Faculty recruitment",
		"Employment Notice for Non-Teaching Posts",
		"Application form for Ph.D. admission",
		"Something completely novel and unheard of",
	}
	results := make([]scraper.Result, 0, len(unmatched))
	for _, s := range unmatched {
		results = append(results, scraper.Result{Title: s})
	}
	got := filterNonAds(results)
	if len(got) != len(unmatched) {
		t.Errorf("expected %d (unmatched kept), got %d", len(unmatched), len(got))
	}
	for _, r := range got {
		if r.Metadata["notice_type"] != "ad" {
			t.Errorf("result %q: notice_type = %q, want %q", r.Title, r.Metadata["notice_type"], "ad")
		}
	}
}

func TestSearch_appliesNonAdAndRecencyFilters(t *testing.T) {
	recent := time.Now().AddDate(0, 0, -5).Format("02-01-2006") // DD-MM-YYYY, 5 days ago
	old := time.Now().AddDate(0, 0, -200).Format("02-01-2006") // 200 days ago

	html := `<table><tbody>
<tr><td><a href="/r1.pdf">Advertisement for Guest Faculty</a></td><td>` + recent + `</td></tr>
<tr><td><a href="/r2.pdf">Schedule of Interview for Assistant Professor</a></td><td>` + recent + `</td></tr>
<tr><td><a href="/r3.pdf">Advertisement for JRF</a></td><td>` + old + `</td></tr>
<tr><td><a href="/r4.pdf">Result of Interview</a></td><td>` + recent + `</td></tr>
</tbody></table>`

	n := IPU{Fetcher: &mockFetcher{html: html}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{JobAge: 30})
	if len(results) != 1 {
		t.Fatalf("expected 1 (recent ad only), got %d: %+v", len(results), results)
	}
	if results[0].Title != "Advertisement for Guest Faculty" {
		t.Errorf("title: got %q", results[0].Title)
	}
	if results[0].Metadata["notice_type"] != "ad" {
		t.Errorf("notice_type: got %q, want %q", results[0].Metadata["notice_type"], "ad")
	}
}
