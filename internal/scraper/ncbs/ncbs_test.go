package ncbs

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

// mockFetcher returns canned HTML for any URL.
type mockFetcher struct {
	html string
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) {
	return m.html, nil
}

// Minimal NCBS listing page with 2 job rows matching the live Drupal Views table structure.
const testHTML = `<html><body>
<table class="views-table cols-10 table table-hover table-striped">
<thead><tr><th>Title</th><th>Qualification</th><th>Domain</th><th>Marks</th><th>Experience</th><th>Category</th><th>Vacancy</th><th>Date</th><th>View</th><th>Apply</th></tr></thead>
<tbody>
<tr class="odd views-row-first">
<td class="views-field views-field-title" >
<a href="/jobportal/online/72026/142669">Advt No.7/2026 : Appointment for the post of Accounts Officer &lsquo;C&rsquo; reserved for OBC category on a permanent basis</a>          </td>
<td class="views-field views-field-php" >
Post Graduate, Graduation          </td>
<td class="views-field views-field-php-1" >
Commerce          </td>
<td class="views-field views-field-field-job-mark" >
60%          </td>
<td class="views-field views-field-php-2" >
6 years          </td>
<td class="views-field views-field-php-3" >
OBC          </td>
<td class="views-field views-field-field-job-vacancy" >
1          </td>
<td class="views-field views-field-field-job-date" >
<span class="date-display-single">20/07/2026</span>          </td>
<td class="views-field views-field-php-4" >
<a class="btn btn-primary" href="/jobportal/node/142669">View</a>          </td>
<td class="views-field views-field-php-5" >
<a class="btn btn-success" href="/jobportal/node/add/application/142669">Apply</a>          </td>
</tr>
<tr class="even">
<td class="views-field views-field-title" >
<a href="/jobportal/online/72026/142672">Advt No.7/2026 : Appointment for the post of Scientific Officer &lsquo;C&rsquo; (Veterinarian) reserved for OBC category on a permanent basis</a>          </td>
<td class="views-field views-field-php" >
B.V.Sc          </td>
<td class="views-field views-field-php-1" >
Veterinary          </td>
<td class="views-field views-field-field-job-mark" >
60%          </td>
<td class="views-field views-field-php-2" >
2 years          </td>
<td class="views-field views-field-php-3" >
OBC          </td>
<td class="views-field views-field-field-job-vacancy" >
1          </td>
<td class="views-field views-field-field-job-date" >
<span class="date-display-single">20/07/2026</span>          </td>
<td class="views-field views-field-php-4" >
<a class="btn btn-primary" href="/jobportal/node/142672">View</a>          </td>
<td class="views-field views-field-php-5" >
<a class="btn btn-success" href="/jobportal/node/add/application/142672">Apply</a>          </td>
</tr>
</tbody>
</table>
</body></html>`

func TestParseJobRows(t *testing.T) {
	results := parseJobRows(testHTML)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result
	r := results[0]
	if r.ID != "142669" {
		t.Errorf("ID: got %q, want 142669", r.ID)
	}
	if r.Company != "NCBS (TIFR)" {
		t.Errorf("Company: got %q, want NCBS (TIFR)", r.Company)
	}
	if r.Location != "Bengaluru, India" {
		t.Errorf("Location: got %q, want Bengaluru, India", r.Location)
	}
	if r.Date != "20/07/2026" {
		t.Errorf("Date: got %q, want 20/07/2026", r.Date)
	}
	if r.URL != "https://www.ncbs.res.in/jobportal/node/142669" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Title == "" {
		t.Error("Title should not be empty")
	}
	if !contains(r.Title, "Accounts Officer") {
		t.Errorf("Title should contain 'Accounts Officer', got %q", r.Title)
	}
	if r.Metadata["qualification"] != "Post Graduate, Graduation" {
		t.Errorf("qualification: got %q", r.Metadata["qualification"])
	}
	if r.Metadata["domain"] != "Commerce" {
		t.Errorf("domain: got %q", r.Metadata["domain"])
	}
	if r.Metadata["experience"] != "6 years" {
		t.Errorf("experience: got %q", r.Metadata["experience"])
	}
	if r.Metadata["reservation"] != "OBC" {
		t.Errorf("reservation: got %q", r.Metadata["reservation"])
	}
	if r.Metadata["vacancy"] != "1" {
		t.Errorf("vacancy: got %q", r.Metadata["vacancy"])
	}

	// Second result
	r2 := results[1]
	if r2.ID != "142672" {
		t.Errorf("second ID: got %q, want 142672", r2.ID)
	}
	if r2.Metadata["domain"] != "Veterinary" {
		t.Errorf("second domain: got %q", r2.Metadata["domain"])
	}
}

func TestParseJobRows_empty(t *testing.T) {
	results := parseJobRows("<html><body><table></table></body></html>")
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty table, got %d", len(results))
	}
}

func TestSearch_withQuery(t *testing.T) {
	n := NCBS{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "veterinarian"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "veterinarian"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'veterinarian', got %d", len(results))
	}
	if results[0].ID != "142672" {
		t.Errorf("expected 142672, got %s", results[0].ID)
	}
}

func TestSearch_withLimit(t *testing.T) {
	n := NCBS{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Limit: 1})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Limit: 1})
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit, got %d", len(results))
	}
}

func TestSearch_noMatch(t *testing.T) {
	n := NCBS{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent query, got %d", len(results))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsFold(s, substr))
}

func containsFold(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc, tc := s[i+j], substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
