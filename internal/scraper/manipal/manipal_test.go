package manipal

import (
	"context"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<html><body>
<table>
<tbody>
<tr><th>Date</th><th>Department</th><th>Position</th><th>Details</th></tr>
<tr>
<td style="text-align: center;">Jul 01, 2026</td>
<td>Department of Biotherapeutics Research (DBR), Manipal</td>
<td>Faculty Positions</td>
<td><a href="/content/dam/manipal/mu/documents/mahe/Careers/j2026/faculty/Faculty%20DBR%20Mpl%20-%20Jul%2001%202026.pdf" target="_blank">Details</a><br />
Apply on or before Aug 20, 2026</td>
</tr>
<tr>
<td>May 13, 2026</td>
<td>Manipal Institute of Technology (MIT), Bangalore</td>
<td>Faculty Positions</td>
<td><a href="/content/dam/manipal/mu/documents/mahe/Careers/j2026/faculty/Faculty%20MIT%20Blr%20-%20May%2013%202026.pdf" target="_blank">Details</a><br />
<br />
</td>
</tr>
</tbody>
</table>
</body></html>`

func TestParseTable(t *testing.T) {
	results := parseTable(testHTML)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}

	r := results[0]
	if r.Title != "Faculty Positions — Department of Biotherapeutics Research (DBR), Manipal" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "Aug 20, 2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.manipal.edu/content/dam/manipal/mu/documents/mahe/Careers/j2026/faculty/Faculty%20DBR%20Mpl%20-%20Jul%2001%202026.pdf" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["posted_date"] != "Jul 01, 2026" {
		t.Errorf("posted_date: got %q", r.Metadata["posted_date"])
	}
	if r.Metadata["department"] != "Department of Biotherapeutics Research (DBR), Manipal" {
		t.Errorf("department: got %q", r.Metadata["department"])
	}

	r2 := results[1]
	if r2.Date != "" {
		t.Errorf("second Date should be empty, got %q", r2.Date)
	}
}

func TestParseTable_empty(t *testing.T) {
	if len(parseTable("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := Manipal{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "faculty"})
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "faculty"})
	// mockFetcher returns same HTML for both faculty and staff pages
	if len(results) != 4 {
		t.Errorf("expected 4 (2 pages × 2 results), got %d", len(results))
	}
}

func TestSearch_noMatch(t *testing.T) {
	n := Manipal{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "nonexistent"})
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
