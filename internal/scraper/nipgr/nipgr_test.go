package nipgr

import (
	"context"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<table>
<tbody>
<tr>
<td class="column-1">Walk-in-Interview for Project Associate-I <a href="">Walk-In</a></td>
<td class="column-2"></td>
<td class="column-3"><a href="https://nipgr.ac.in/nipgrv4/wp-content/uploads/2026/06/Advt.pdf" download="">VIEW</a></td>
<td class="column-4"><a href="https://nipgr.ac.in/nipgrv4/wp-content/uploads/2026/06/AppForm.docx" download="">APPLICATION FORMAT</a></td>
</tr>
<tr>
<td class="column-1">Research Associate-I (two posts)</td>
<td class="column-2">22/06/2026</td>
<td class="column-3"><a href="https://nipgr.ac.in/nipgrv4/wp-content/uploads/2026/06/RA_Advt.pdf" download="">VIEW</a></td>
<td class="column-4"></td>
</tr>
</tbody>
</table>`

func TestParseTable(t *testing.T) {
	results := parseTable(testHTML)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	r := results[0]
	if r.Title != "Walk-in-Interview for Project Associate-I Walk-In" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.URL != "https://nipgr.ac.in/nipgrv4/wp-content/uploads/2026/06/Advt.pdf" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["application_form"] != "https://nipgr.ac.in/nipgrv4/wp-content/uploads/2026/06/AppForm.docx" {
		t.Errorf("application_form: got %q", r.Metadata["application_form"])
	}

	r2 := results[1]
	if r2.Date != "22/06/2026" {
		t.Errorf("second Date: got %q", r2.Date)
	}
}

func TestParseTable_empty(t *testing.T) {
	if len(parseTable("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := NIPGR{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "research"})
	results = scraper.ApplyFilters(results, scraper.SearchOpts{Query: "research"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}
