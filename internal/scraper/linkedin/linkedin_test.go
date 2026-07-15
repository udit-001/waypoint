package linkedin

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

type mockFetcher struct{ html string }

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) { return m.html, nil }

type urlCapturingFetcher struct{ html, gotURL string }

func (m *urlCapturingFetcher) Fetch(_ context.Context, fetchURL string) (string, error) {
	m.gotURL = fetchURL
	return m.html, nil
}

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

func TestWorkTypeFlag(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"remote", "2"},
		{"hybrid", "3"},
		{"onsite", "1"},
		{"on-site", "1"},
		{"", ""},
		{"bogus", ""},
	}
	for _, c := range cases {
		if got := workTypeFlag(c.input); got != c.want {
			t.Errorf("workTypeFlag(%q): got %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSearch_jobAgeSetsTPR(t *testing.T) {
	f := &urlCapturingFetcher{html: testHTML}
	n := LinkedIn{Fetcher: f}
	_, _ = n.Search(context.Background(), scraper.SearchOpts{
		Query:    "scientist",
		Location: "India",
		JobAge:   7,
	})
	params := urlParams(t, f.gotURL)
	if params.Get("f_TPR") != "r604800" {
		t.Errorf("f_TPR: got %q, want r604800", params.Get("f_TPR"))
	}
}

func TestSearch_remoteSetsWT(t *testing.T) {
	f := &urlCapturingFetcher{html: testHTML}
	n := LinkedIn{Fetcher: f}
	_, _ = n.Search(context.Background(), scraper.SearchOpts{
		Remote: "remote",
	})
	params := urlParams(t, f.gotURL)
	if params.Get("f_WT") != "2" {
		t.Errorf("f_WT: got %q, want 2", params.Get("f_WT"))
	}
}

func TestSearch_pageSetsStart(t *testing.T) {
	f := &urlCapturingFetcher{html: testHTML}
	n := LinkedIn{Fetcher: f}
	_, _ = n.Search(context.Background(), scraper.SearchOpts{
		Page: 3,
	})
	params := urlParams(t, f.gotURL)
	if params.Get("start") != "20" {
		t.Errorf("start: got %q, want 20", params.Get("start"))
	}
}

func TestSearch_defaultPageStart0(t *testing.T) {
	f := &urlCapturingFetcher{html: testHTML}
	n := LinkedIn{Fetcher: f}
	_, _ = n.Search(context.Background(), scraper.SearchOpts{})
	params := urlParams(t, f.gotURL)
	if params.Get("start") != "0" {
		t.Errorf("start: got %q, want 0", params.Get("start"))
	}
}

func TestSearch_noKeywordsWhenQueryEmpty(t *testing.T) {
	f := &urlCapturingFetcher{html: testHTML}
	n := LinkedIn{Fetcher: f}
	_, _ = n.Search(context.Background(), scraper.SearchOpts{
		Location: "India",
	})
	params := urlParams(t, f.gotURL)
	if _, ok := params["keywords"]; ok {
		t.Error("keywords should not be set when query is empty")
	}
}

func urlParams(t *testing.T, rawURL string) url.Values {
	t.Helper()
	idx := strings.Index(rawURL, "?")
	if idx < 0 {
		t.Fatalf("no query params in URL %q", rawURL)
	}
	v, err := url.ParseQuery(rawURL[idx+1:])
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	return v
}

const testDetailHTML = `<div class="topcard">
<h1 class="top-card-layout__title">Food Scientist</h1>
<a class="topcard__org-name-link" href="https://www.linkedin.com/company/griffith-foods">Griffith Foods</a>
<span class="topcard__flavor topcard__flavor--bullet">Bengaluru, Karnataka, India</span>
</div>
<div class="description__text">
<h3 class="description__job-criteria-subheader">Seniority level</h3>
<span class="description__job-criteria-text">Not Applicable</span>
<h3 class="description__job-criteria-subheader">Employment type</h3>
<span class="description__job-criteria-text">Full-time</span>
<h3 class="description__job-criteria-subheader">Job function</h3>
<span class="description__job-criteria-text">Research</span>
<h3 class="description__job-criteria-subheader">Industries</h3>
<span class="description__job-criteria-text">Food and Beverage Manufacturing</span>
<div class="show-more-less-html__markup">
<p>Griffith Foods is the caring, creative product development partner.</p>
<p>We are looking for a Food Scientist to join our team.</p>
</div>
</div>`

func TestParseJobDetail(t *testing.T) {
	r := parseJobDetail(testDetailHTML, "4428289369")

	if r.ID != "4428289369" {
		t.Errorf("ID: got %q", r.ID)
	}
	if r.Title != "Food Scientist" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Company != "Griffith Foods" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Location != "Bengaluru, Karnataka, India" {
		t.Errorf("Location: got %q", r.Location)
	}
	if r.URL != "https://www.linkedin.com/jobs/view/4428289369" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Description == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(r.Description, "Griffith Foods is the caring") {
		t.Errorf("Description should contain expected text, got %q", r.Description[:80])
	}
	if r.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if r.Metadata["seniority level"] != "Not Applicable" {
		t.Errorf("seniority level: got %q", r.Metadata["seniority level"])
	}
	if r.Metadata["employment type"] != "Full-time" {
		t.Errorf("employment type: got %q", r.Metadata["employment type"])
	}
	if r.Metadata["job function"] != "Research" {
		t.Errorf("job function: got %q", r.Metadata["job function"])
	}
	if r.Metadata["industries"] != "Food and Beverage Manufacturing" {
		t.Errorf("industries: got %q", r.Metadata["industries"])
	}
}

func TestNormalizeID(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"4439995582", "4439995582"},
		{"https://www.linkedin.com/jobs/view/4439995582", "4439995582"},
		{"https://in.linkedin.com/jobs/view/associate-scientist-at-syngene-4439995582", "4439995582"},
		{"urn:li:jobPosting:4439995582", "4439995582"},
		{"not-a-valid-id", ""},
		{"123", ""},
	}
	for _, c := range cases {
		if got := normalizeID(c.input); got != c.want {
			t.Errorf("normalizeID(%q): got %q, want %q", c.input, got, c.want)
		}
	}
}

func TestDetail(t *testing.T) {
	n := LinkedIn{Fetcher: &mockFetcher{html: testDetailHTML}}
	r, err := n.Detail(context.Background(), "4428289369")
	if err != nil {
		t.Fatalf("Detail failed: %v", err)
	}
	if r.Title != "Food Scientist" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Metadata["employment type"] != "Full-time" {
		t.Errorf("employment type: got %q", r.Metadata["employment type"])
	}
}

func TestDetail_acceptsURL(t *testing.T) {
	n := LinkedIn{Fetcher: &mockFetcher{html: testDetailHTML}}
	r, err := n.Detail(context.Background(), "https://www.linkedin.com/jobs/view/4428289369")
	if err != nil {
		t.Fatalf("Detail failed: %v", err)
	}
	if r.ID != "4428289369" {
		t.Errorf("ID: got %q", r.ID)
	}
}

func TestDetail_invalidID(t *testing.T) {
	n := LinkedIn{Fetcher: &mockFetcher{html: testDetailHTML}}
	_, err := n.Detail(context.Background(), "not-a-valid-id")
	if err == nil {
		t.Error("expected error for invalid ID")
	}
}

func TestDetail_notFound(t *testing.T) {
	n := LinkedIn{Fetcher: &mockFetcher{html: ""}}
	_, err := n.Detail(context.Background(), "9999999999")
	if err == nil {
		t.Error("expected error for not-found job")
	}
}
