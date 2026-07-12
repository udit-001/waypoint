package ipu

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
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
