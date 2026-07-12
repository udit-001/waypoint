package iisc

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

type mockFetcher struct {
	html string
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) {
	return m.html, nil
}

const testHTML = `<html><body>
<table>
<thead><tr class="row-1">
<th class="column-1">Sl. No.</th><th class="column-2">Advt. No.</th><th class="column-3">Post</th><th class="column-4">Department</th><th class="column-5">Detailed Advertisement</th><th class="column-6">Application Start Date</th><th class="column-7">Application Closing Date</th><th class="column-8">Online Application Link</th><th class="column-9">Updates</th><th class="column-10">Status of Recruitment</th>
</tr></thead>
<tbody>
<tr class="row-2">
<td class="column-1">122</td><td class="column-2">R(HR)Temp-11(PSW-2-WC)/2026</td><td class="column-3">Psychiatric Social Worker</td><td class="column-4">Wellness Centre</td><td class="column-5"><a href="/wp-content/uploads/2026/06/2.1-Advertisement-PSW.pdf">Click here</a></td><td class="column-6">29.06.2026</td><td class="column-7">20.07.2026</td><td class="column-8"><a href="https://recruitment.iisc.ac.in/Temporary_Positions/">Link</a></td><td class="column-9"></td><td class="column-10">Open</td>
</tr>
<tr class="row-3">
<td class="column-1">121</td><td class="column-2">R(HR)Temp-08(SA-UGP)/2026</td><td class="column-3">System Administrator</td><td class="column-4">Undergraduate Programme</td><td class="column-5"><a href="/wp-content/uploads/2026/06/2.-HR-Advertisement-SA.pdf">Click here</a></td><td class="column-6">19.06.2026</td><td class="column-7">10.07.2026</td><td class="column-8"><a href="https://recruitment.iisc.ac.in/Temporary_Positions/">Link</a></td><td class="column-9"></td><td class="column-10">Closed</td>
</tr>
</tbody>
</table>
</body></html>`

func TestParseJobRows(t *testing.T) {
	results := parseJobRows(testHTML)

	if len(results) != 1 {
		t.Fatalf("expected 1 Open result (1 Closed filtered), got %d", len(results))
	}

	r := results[0]
	if r.ID != "122" {
		t.Errorf("ID: got %q, want 122", r.ID)
	}
	if r.Title != "Psychiatric Social Worker" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Company != "IISc" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Date != "20.07.2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.iisc.ac.in/wp-content/uploads/2026/06/2.1-Advertisement-PSW.pdf" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["advt_no"] != "R(HR)Temp-11(PSW-2-WC)/2026" {
		t.Errorf("advt_no: got %q", r.Metadata["advt_no"])
	}
	if r.Metadata["department"] != "Wellness Centre" {
		t.Errorf("department: got %q", r.Metadata["department"])
	}
	if r.Metadata["start_date"] != "29.06.2026" {
		t.Errorf("start_date: got %q", r.Metadata["start_date"])
	}
	if r.Metadata["application_link"] != "https://recruitment.iisc.ac.in/Temporary_Positions/" {
		t.Errorf("application_link: got %q", r.Metadata["application_link"])
	}
}

func TestParseJobRows_empty(t *testing.T) {
	results := parseJobRows("<html><body><table></table></body></html>")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestSearch_withQuery(t *testing.T) {
	n := IISc{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "social"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "social"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestSearch_noMatch(t *testing.T) {
	n := IISc{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
