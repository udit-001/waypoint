package jncasr

import (
	"context"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

type mockFetcher struct {
	html string
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) {
	return m.html, nil
}

const testHTML = `<html><body>
<table class="views-table">
<thead><tr><th>Sl. No.</th><th>Title</th><th>Posted</th><th>Due Date</th><th>Download</th></tr></thead>
<tbody>
<tr>
<td class="views-field views-field-counter">1</td>
<td class="views-field views-field-title">
<!-- THEME DEBUG --><!-- BEGIN OUTPUT -->
<div id="1"><a href="/openings/notification" hreflang="en">Notification</a></div>
<i>Type: Administrative Staff</i>
<!-- END OUTPUT -->
</td>
<td class="views-field views-field-field-in-date">
<!-- DEBUG -->10/07/2026<!-- END -->
</td>
<td class="views-field views-field-field-end-date">
<!-- DEBUG -->25/07/2026<!-- END -->
</td>
<td class="views-field views-field-views-conditional-field">
<a href="/sites/default/files/Join_Us/test.pdf">Notification PDF</a>
</td>
</tr>
<tr>
<td class="views-field views-field-counter">2</td>
<td class="views-field views-field-title">
<div id="2"><a href="/openings/junior-research-fellow-9" hreflang="en">Junior Research Fellow</a></div>
<i>Type: Research Associate</i>
</td>
<td class="views-field views-field-field-in-date">05/07/2026</td>
<td class="views-field views-field-field-end-date">20/07/2026</td>
<td class="views-field views-field-views-conditional-field">
<a href="/sites/default/files/jrf.pdf">JRF Advertisement</a>
</td>
</tr>
</tbody>
</table>
</body></html>`

func TestParseJobRows(t *testing.T) {
	results := parseJobRows(testHTML)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	r := results[0]
	if r.ID != "notification" {
		t.Errorf("ID: got %q, want notification", r.ID)
	}
	if r.Company != "JNCASR" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Date != "25/07/2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.Metadata["posted_date"] != "10/07/2026" {
		t.Errorf("posted_date: got %q", r.Metadata["posted_date"])
	}
	if !contains(r.Title, "Notification") {
		t.Errorf("Title should contain 'Notification', got %q", r.Title)
	}
	if !contains(r.Title, "Administrative Staff") {
		t.Errorf("Title should contain type annotation, got %q", r.Title)
	}

	r2 := results[1]
	if r2.ID != "junior-research-fellow-9" {
		t.Errorf("second ID: got %q", r2.ID)
	}
	if !contains(r2.Title, "Junior Research Fellow") {
		t.Errorf("second Title: got %q", r2.Title)
	}
}

func TestParseJobRows_empty(t *testing.T) {
	results := parseJobRows("<html><body><table></table></body></html>")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestSearch_withQuery(t *testing.T) {
	n := JNCASR{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "research"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "research"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
	if results[0].ID != "junior-research-fellow-9" {
		t.Errorf("expected junior-research-fellow-9, got %s", results[0].ID)
	}
}

func TestSearch_noMatch(t *testing.T) {
	n := JNCASR{Fetcher: &mockFetcher{html: testHTML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "nonexistent"})
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
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
