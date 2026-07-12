package nabi

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<table>
<tbody>
<tr>
<td class="text-center" scope="row">1</td>
<td>Walk-in Online Interview for the temporary Position of Project Associate(02)</td>
<td class="text-center">06 Jul 2026 09:00 AM</td>
<td class="text-center datafiles">
<a href="https://nabi.res.in/backend/web/img/jobs/jobs-547-Advt5.pdf" target="_blank" title="jobs-547-Advertisement 5 of 2026">
<img src="/frontend/web/images/pdf.png" alt="Download pdf" class="img-responsive">
<span>( 356.77 KB )</span>
</a>
</td>
</tr>
<tr>
<td class="text-center" scope="row">2</td>
<td>Applications Invited under PMRC scheme 2026</td>
<td class="text-center">02 Jul 2026 05:00 PM</td>
<td class="text-center datafiles">
<a href="https://nabi.res.in/backend/web/img/jobs/jobs-544-PMRC.pdf" target="_blank" title="jobs-544-Advt - PMRC">
<img src="/frontend/web/images/pdf.png" alt="Download pdf" class="img-responsive">
<span>( 200.45 KB )</span>
</a>
</td>
</tr>
</tbody>
</table>`

func TestParseTable(t *testing.T) {
	results := parseTable(testHTML)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	r := results[0]
	if r.Title != "Walk-in Online Interview for the temporary Position of Project Associate(02)" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "06 Jul 2026 09:00 AM" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://nabi.res.in/backend/web/img/jobs/jobs-547-Advt5.pdf" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.ID != "1" {
		t.Errorf("ID: got %q", r.ID)
	}
	if r.Location != "Mohali, India" {
		t.Errorf("Location: got %q", r.Location)
	}
}

func TestParseTable_empty(t *testing.T) {
	if len(parseTable("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := NABI{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "project"})
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "project"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}
