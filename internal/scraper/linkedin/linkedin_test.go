package linkedin

import (
	"context"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

const testHTML = `<ul>
<li>
<div data-entity-urn="urn:li:jobPosting:4432186738">
<a class="base-card__full-link" href="https://www.linkedin.com/jobs/view/4432186738?refId=abc">View</a>
<h3 class="base-search-card__title">Development Scientist I</h3>
<h4 class="base-search-card__subtitle"><a href="/company/beckman">Beckman Coulter</a></h4>
<span class="job-search-card__location">Bengaluru, Karnataka, India</span>
<time class="job-search-card__listdate" datetime="2026-06-25">25 Jun 2026</time>
</div>
</li>
<li>
<div data-entity-urn="urn:li:jobPosting:4438185458">
<a class="base-card__full-link" href="https://www.linkedin.com/jobs/view/4438185458">View</a>
<h3 class="base-search-card__title">Research Engineer - Technical Writer</h3>
<h4 class="base-search-card__subtitle">Aurora</h4>
<span class="job-search-card__location">Bengaluru, Karnataka, India</span>
<time class="job-search-card__listdate" datetime="2026-07-08">8 Jul 2026</time>
</div>
</li>
</ul>`

func TestParseJobCards(t *testing.T) {
	results := parseJobCards(testHTML)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	r := results[0]
	if r.ID != "4432186738" {
		t.Errorf("ID: got %q", r.ID)
	}
	if r.Title != "Development Scientist I" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Company != "Beckman Coulter" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Location != "Bengaluru, Karnataka, India" {
		t.Errorf("Location: got %q", r.Location)
	}
	if r.Date != "2026-06-25" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.linkedin.com/jobs/view/4432186738" {
		t.Errorf("URL: got %q", r.URL)
	}
}

func TestParseJobCards_empty(t *testing.T) {
	if len(parseJobCards("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}

func TestSearch_defaultLocation(t *testing.T) {
	n := LinkedIn{Fetcher: &mockFetcher{html: testHTML}}
	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "research"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	// LinkedIn's API does the keyword filtering; the mock returns all 2 results
	if len(results) != 2 {
		t.Errorf("expected 2 results (mock doesn't filter), got %d", len(results))
	}
}

func TestSearch_withLocation(t *testing.T) {
	n := LinkedIn{Fetcher: &mockFetcher{html: testHTML}}
	results, err := n.Search(context.Background(), scraper.SearchOpts{
		Query:    "scientist",
		Location: "Bengaluru, India",
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (mock doesn't filter), got %d", len(results))
	}
}

func TestJobageToTPR(t *testing.T) {
	if jobageToTPR(7) != "r604800" {
		t.Errorf("7 days: got %q", jobageToTPR(7))
	}
	if jobageToTPR(0) != "" {
		t.Errorf("0 days should be empty")
	}
	if jobageToTPR(9999) != "" {
		t.Errorf("9999 days should be empty")
	}
}
