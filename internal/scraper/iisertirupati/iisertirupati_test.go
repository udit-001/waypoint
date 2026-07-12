package iisertirupati

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
<td>1</td>
<td><div class="sjb-with-logo"><div class="job-without-company"><h4>
<a href="https://www.iisertirupati.ac.in/jobs/advt_502026/">
<span class="job-title">Advertisement No.: (50/2026) – Recruitment for Technical Officer</span>
</a></h4></div></div></td>
<td>July 20, 2026</td>
<td><div class="sjb-apply-now-btn"><p><a href="https://www.iisertirupati.ac.in/jobs/advt_502026/" class="btn btn-primary">Click Here</a></p></div></td>
</tr>
</tbody>
</table>`

func TestParseJobs(t *testing.T) {
	results := parseJobs(testHTML)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	r := results[0]
	if r.Title != "Advertisement No.: (50/2026) – Recruitment for Technical Officer" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "July 20, 2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.iisertirupati.ac.in/jobs/advt_502026/" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.ID != "advt_502026" {
		t.Errorf("ID: got %q", r.ID)
	}
}

func TestParseJobs_empty(t *testing.T) {
	if len(parseJobs("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := IISERTirupati{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "technical"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}
