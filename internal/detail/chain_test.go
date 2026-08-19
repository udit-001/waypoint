package detail

import (
	"context"
	"fmt"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// --- fakes ---

// fakeBoard implements BoardDetailer for testing.
type fakeBoard struct {
	results map[string]scraper.Result
	err     error
}

func (f *fakeBoard) Detail(ctx context.Context, boardURL, id string) (scraper.Result, error) {
	if f.err != nil {
		return scraper.Result{}, f.err
	}
	r, ok := f.results[id]
	if !ok {
		return scraper.Result{}, fmt.Errorf("not found: %s", id)
	}
	return r, nil
}

// fakeScraperDetail implements ScraperDetailer for testing.
type fakeScraperDetail struct {
	results map[string]*scraper.Result
	err     error
}

func (f *fakeScraperDetail) Detail(ctx context.Context, id string) (*scraper.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	r, ok := f.results[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return r, nil
}

// fakeExa implements ExaFetcher for testing.
type fakeExa struct {
	bodies map[string]string // URL → markdown body
	err    error
}

func (f *fakeExa) Fetch(ctx context.Context, rawURL string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	body, ok := f.bodies[rawURL]
	if !ok {
		return "", fmt.Errorf("not found: %s", rawURL)
	}
	return body, nil
}

// --- chain tests ---

func TestChain_Tier1BoardSuccess(t *testing.T) {
	board := &fakeBoard{
		results: map[string]scraper.Result{
			"123": {
				ID:          "123",
				Title:       "Engineer",
				Company:     "Acme",
				URL:         "https://boards.greenhouse.io/acme/jobs/123",
				Description: "Full description from Greenhouse API",
				Metadata:    map[string]string{"department": "eng"},
			},
		},
	}

	chain := NewChain(board, nil, nil, nil, nil)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://boards.greenhouse.io/acme/jobs/123",
		"https://boards-api.greenhouse.io/v1/boards/acme/jobs",
		"123",
		"short desc",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "board" {
		t.Errorf("source = %q, want board", result.Source)
	}
	if result.Description != "Full description from Greenhouse API" {
		t.Errorf("description = %q", result.Description)
	}
	if result.Metadata["department"] != "eng" {
		t.Errorf("department = %q", result.Metadata["department"])
	}
}

func TestChain_Tier2ScraperFallback(t *testing.T) {
	// Board fails, scraper succeeds.
	board := &fakeBoard{err: fmt.Errorf("board unavailable")}
	scraperD := &fakeScraperDetail{
		results: map[string]*scraper.Result{
			"456": {
				ID:          "456",
				Title:       "Designer",
				Company:     "Beta",
				Description: "Description from scraper detail",
			},
		},
	}

	chain := NewChain(board, scraperD, nil, nil, nil)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://jobs.lever.co/beta/456",
		"https://jobs.lever.co/beta",
		"456",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "scraper" {
		t.Errorf("source = %q, want scraper", result.Source)
	}
	if result.Description != "Description from scraper detail" {
		t.Errorf("description = %q", result.Description)
	}
}

func TestChain_Tier3ExaFallback(t *testing.T) {
	// Board and scraper fail, Exa succeeds with parsed fields.
	board := &fakeBoard{err: fmt.Errorf("no board")}
	scraperD := &fakeScraperDetail{err: fmt.Errorf("no scraper")}
	exa := &fakeExa{
		bodies: map[string]string{
			"https://boards.greenhouse.io/acme/jobs/789": "# Senior Engineer\n\nat Acme Corp\n\nPay range: $120,000 – $150,000\n\n## Requirements\n\n* 5+ years Go\n* Distributed systems",
		},
	}

	chain := NewChain(board, scraperD, nil, nil, exa)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://boards.greenhouse.io/acme/jobs/789",
		"https://boards.greenhouse.io/acme/jobs",
		"789",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "exa" {
		t.Errorf("source = %q, want exa", result.Source)
	}
	if result.Title != "Senior Engineer" {
		t.Errorf("title = %q, want Senior Engineer", result.Title)
	}
	if result.Salary != "$120,000 – $150,000" {
		t.Errorf("salary = %q", result.Salary)
	}
}

func TestChain_Tier4RawFallback(t *testing.T) {
	// All higher tiers fail — raw fallback.
	board := &fakeBoard{err: fmt.Errorf("no board")}
	scraperD := &fakeScraperDetail{err: fmt.Errorf("no scraper")}
	exa := &fakeExa{err: fmt.Errorf("no exa")}

	chain := NewChain(board, scraperD, nil, nil, exa)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://example.com/jobs/123",
		"",
		"123",
		"existing description",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "none" {
		t.Errorf("source = %q, want none", result.Source)
	}
	if result.Body != "existing description" {
		t.Errorf("body = %q, want existing description", result.Body)
	}
}

func TestChain_SkipsLinkedInInExaTier(t *testing.T) {
	// LinkedIn URL should skip the Exa tier even when Exa is available.
	board := &fakeBoard{err: fmt.Errorf("no board")}
	scraperD := &fakeScraperDetail{err: fmt.Errorf("no scraper")}
	exa := &fakeExa{
		bodies: map[string]string{
			"https://www.linkedin.com/jobs/view/123": "# Should not be fetched",
		},
	}

	chain := NewChain(board, scraperD, nil, nil, exa)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://www.linkedin.com/jobs/view/123",
		"",
		"123",
		"existing desc",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall through to raw (tier 4), not Exa.
	if result.Source != "none" {
		t.Errorf("source = %q, want none (LinkedIn skipped Exa)", result.Source)
	}
}

func TestChain_SkipsExaWhenBodyFilled(t *testing.T) {
	// Exa should be skipped when body fields are already filled.
	board := &fakeBoard{err: fmt.Errorf("no board")}
	scraperD := &fakeScraperDetail{err: fmt.Errorf("no scraper")}
	exa := &fakeExa{
		bodies: map[string]string{
			"https://example.com/jobs/123": "# Should not be fetched",
		},
	}

	chain := NewChain(board, scraperD, nil, nil, exa)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://example.com/jobs/123",
		"",
		"123",
		"existing description",
		map[string]string{"salary": "$100k", "deadline": "2026-01-01"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "none" {
		t.Errorf("source = %q, want none (body already filled)", result.Source)
	}
}

func TestChain_ExaCapRespected(t *testing.T) {
	board := &fakeBoard{err: fmt.Errorf("no board")}
	scraperD := &fakeScraperDetail{err: fmt.Errorf("no scraper")}
	exa := &fakeExa{
		bodies: map[string]string{
			"https://example.com/jobs/1": "# Job 1",
			"https://example.com/jobs/2": "# Job 2",
			"https://example.com/jobs/3": "# Job 3",
		},
	}

	chain := NewChain(board, scraperD, nil, nil, exa)
	chain.SetExaCap(2)

	// First two should use Exa.
	for i := 1; i <= 2; i++ {
		url := fmt.Sprintf("https://example.com/jobs/%d", i)
		result, err := chain.FetchDetail(context.Background(), url, "", fmt.Sprintf("%d", i), "", nil)
		if err != nil {
			t.Fatalf("job %d: unexpected error: %v", i, err)
		}
		if result.Source != "exa" {
			t.Errorf("job %d: source = %q, want exa", i, result.Source)
		}
	}

	// Third should be capped — falls to raw.
	result, err := chain.FetchDetail(context.Background(), "https://example.com/jobs/3", "", "3", "existing", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "none" {
		t.Errorf("job 3: source = %q, want none (exa capped)", result.Source)
	}
}

func TestChain_ParserFamilyDetection(t *testing.T) {
	board := &fakeBoard{err: fmt.Errorf("no board")}
	scraperD := &fakeScraperDetail{err: fmt.Errorf("no scraper")}
	exa := &fakeExa{
		bodies: map[string]string{
			"https://jobs.lever.co/test/abc": "TestCo - Backend Engineer\n\n$130,000 – $170,000 per year",
		},
	}

	chain := NewChain(board, scraperD, nil, nil, exa)
	result, err := chain.FetchDetail(
		context.Background(),
		"https://jobs.lever.co/test/abc",
		"https://jobs.lever.co/test",
		"abc",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "exa" {
		t.Errorf("source = %q, want exa", result.Source)
	}
	// Lever parser should extract title and company from "Company - Title" format.
	if result.Company != "TestCo" {
		t.Errorf("company = %q, want TestCo", result.Company)
	}
	if result.Title != "Backend Engineer" {
		t.Errorf("title = %q, want Backend Engineer", result.Title)
	}
}

// --- merge integration tests ---

func TestChain_MergeIntegration(t *testing.T) {
	board := &fakeBoard{
		results: map[string]scraper.Result{
			"100": {
				ID:          "100",
				Title:       "SRE",
				Company:     "BigCo",
				URL:         "https://boards.greenhouse.io/bigco/jobs/100",
				Date:        "2026-08-01",
				Description: "Full SRE description from board API",
				Location:    "Remote",
				Metadata:    map[string]string{"department": "platform"},
			},
		},
	}

	chain := NewChain(board, nil, nil, nil, nil)
	detail, err := chain.FetchDetail(
		context.Background(),
		"https://boards.greenhouse.io/bigco/jobs/100",
		"https://boards-api.greenhouse.io/v1/boards/bigco/jobs",
		"100",
		"short listing desc",
		map[string]string{"existing": "value"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Merge with listing fields.
	listing := scraper.Result{
		ID:       "100",
		Title:    "SRE (listing title)",
		Company:  "BigCo (listing)",
		URL:      "https://boards.greenhouse.io/bigco/jobs/100",
		Date:     "2026-08-01",
		Location: "Listing, NY",
	}
	merged := Merge(listing, detail, "short listing desc", map[string]string{"existing": "value"})

	// Listing wins for title/company.
	if merged.Title != "SRE (listing title)" {
		t.Errorf("title = %q, want listing title", merged.Title)
	}
	if merged.Company != "BigCo (listing)" {
		t.Errorf("company = %q, want listing company", merged.Company)
	}
	// Detail wins for description.
	if merged.Description != "Full SRE description from board API" {
		t.Errorf("description = %q", merged.Description)
	}
	// Detail wins for location.
	if merged.Location != "Remote" {
		t.Errorf("location = %q, want Remote", merged.Location)
	}
	// Metadata union.
	if merged.Metadata["existing"] != "value" {
		t.Error("existing metadata lost")
	}
	if merged.Metadata["department"] != "platform" {
		t.Errorf("department = %q", merged.Metadata["department"])
	}
	if merged.Metadata["detail_source"] != "board" {
		t.Errorf("detail_source = %q", merged.Metadata["detail_source"])
	}
}
