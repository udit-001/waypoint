package instem

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
<table class="views-table cols-5 table table-hover table-striped">
<thead><tr><th>Job Title</th><th>Vacancies</th><th>Last Date to Apply</th><th>View</th><th>Apply</th></tr></thead>
<tbody>
<tr class="odd views-row-first">
<td class="views-field views-field-title" >
<a href="/jobportal/bric-instem012026/129141">Administrative Officer</a>          </td>
<td class="views-field views-field-field-job-vacancy" >
1          </td>
<td class="views-field views-field-field-job-date" >
<span class="date-display-single">13-Jul-2026</span>          </td>
<td class="views-field views-field-php-4" >
<a class="btn btn-primary" href="/jobportal/node/129141">View</a>          </td>
<td class="views-field views-field-php-5" >
<a class="btn btn-success" href="/jobportal/node/add/application/129141">Apply</a>          </td>
</tr>
</tbody>
</table>
</body></html>`

func TestParseJobRows(t *testing.T) {
	results := parseJobRows(testHTML)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.ID != "129141" {
		t.Errorf("ID: got %q, want 129141", r.ID)
	}
	if r.Company != "inStem (BRIC)" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Location != "Bengaluru, India" {
		t.Errorf("Location: got %q", r.Location)
	}
	if r.Date != "13-Jul-2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.instem.res.in/jobportal/node/129141" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Title != "Administrative Officer" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Metadata["vacancy"] != "1" {
		t.Errorf("vacancy: got %q", r.Metadata["vacancy"])
	}
}

func TestParseJobRows_empty(t *testing.T) {
	results := parseJobRows("<html><body><table></table></body></html>")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestSearch_withQuery(t *testing.T) {
	n := InStem{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "admin"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "admin"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestSearch_noMatch(t *testing.T) {
	n := InStem{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
