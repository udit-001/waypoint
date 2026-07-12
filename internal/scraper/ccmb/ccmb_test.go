package ccmb

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<html><body>
<div class="card mb-4 notification-card" data-category="Project Positions">
  <div class="card-body">
    <h4 class="card-title">Notification No.0626/A for various project position(s)</h4>
    <div class="notification-date">
      <div class="row meta-row">
        <div class="col-md-4"><p><strong>Last Date to Apply:</strong> 01/07/2026</p></div>
        <div class="col-md-4"><p><strong>Posted Date:</strong> 17/06/2026</p></div>
        <div class="col-md-4"><p><strong>Category:</strong> Project Positions</p></div>
      </div>
    </div>
    <div class="job-action-btn">
      <p><a href="https://ccmb.res.in/wp-content/uploads/2026/06/0626A_Notification.pdf">Click here</a></p>
    </div>
  </div>
</div>
</body></html>`

func TestParseCards(t *testing.T) {
	results := parseCards(testHTML)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	r := results[0]
	if r.Title != "Notification No.0626/A for various project position(s)" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "01/07/2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.Metadata["posted_date"] != "17/06/2026" {
		t.Errorf("posted_date: got %q", r.Metadata["posted_date"])
	}
	if r.Metadata["category"] != "Project Positions" {
		t.Errorf("category: got %q", r.Metadata["category"])
	}
	if r.URL != "https://ccmb.res.in/wp-content/uploads/2026/06/0626A_Notification.pdf" {
		t.Errorf("URL: got %q", r.URL)
	}
}

func TestParseCards_empty(t *testing.T) {
	if len(parseCards("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_query(t *testing.T) {
	n := CCMB{Fetcher: &mockFetcher{html: testHTML}}
	results, _ := n.Search(context.Background(), scraper.SearchOpts{Query: "project"})
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}
