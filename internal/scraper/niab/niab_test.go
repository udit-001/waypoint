package niab

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<html><body>
<div class="elementor-text-editor">
<ul>
<li><a href="http://www.niab.org.in/Notifications_15_2026.aspx" target="_blank">Recruitment of Field Assistant <em><span style="color: #3366ff;">(Advt. No. 15/2026)</span></em></a></li>
<li><a href="https://www.niab.res.in/wp-content/uploads/2026/06/Notifications_14_2026.pdf" target="_blank">Walk-in Interview (Project Associate-II) <em><span style="color: #3366ff;">(Advt. No. 14/2026)</span></em></a></li>
<li><a href="http://www.niab.org.in/Notifications_10_2026.aspx" target="_blank">Recruitment of Project Associate I</a> <a href="result.pdf">– Result</a></li>
</ul>
</div>
</body></html>`

func TestParseListings(t *testing.T) {
	results := parseListings(testHTML)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	r := results[0]
	if r.Title != "Recruitment of Field Assistant (Advt. No. 15/2026)" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.URL != "http://www.niab.org.in/Notifications_15_2026.aspx" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["advt_no"] != "15/2026" {
		t.Errorf("advt_no: got %q", r.Metadata["advt_no"])
	}
	if r.Company != "NIAB" {
		t.Errorf("Company: got %q", r.Company)
	}
}

func TestParseListings_empty(t *testing.T) {
	if len(parseListings("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := NIAB{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "project"})
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "project"})
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}
