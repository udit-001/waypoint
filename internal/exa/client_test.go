package exa

import (
	"context"
	"strings"
	"testing"
)

func TestSearchCompanies_parsesJSONShape(t *testing.T) {
	c := New("", nil)
	c.callTool = func(_ context.Context, tool string, _ map[string]any) (string, error) {
		if tool != "web_search_exa" {
			t.Errorf("tool = %q, want web_search_exa", tool)
		}
		return `[{"title":"Stripe","url":"https://stripe.com"},{"title":"Ramp","url":"https://ramp.com"}]`, nil
	}
	hits, err := c.SearchCompanies(context.Background(), "payments", 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Name != "Stripe" || hits[0].URL != "https://stripe.com" {
		t.Errorf("hits = %+v", hits)
	}
}

func TestSearchCompanies_parsesLineShape(t *testing.T) {
	c := New("", nil)
	c.callTool = func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "Title: Ghost\nURL: https://boards.greenhouse.io/ghost\n\nTitle: Plaid\nURL: https://plaid.com", nil
	}
	hits, err := c.SearchCompanies(context.Background(), "fintech", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[1].Name != "Plaid" {
		t.Errorf("hits = %+v", hits)
	}
}

func TestSearchCompanies_cachesPerFacet(t *testing.T) {
	c := New("", nil)
	calls := 0
	c.callTool = func(_ context.Context, _ string, _ map[string]any) (string, error) {
		calls++
		return `[{"title":"Once","url":"https://once.com"}]`, nil
	}
	for i := 0; i < 3; i++ {
		if _, err := c.SearchCompanies(context.Background(), "same-facet", 5); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (cached)", calls)
	}
}

func TestParseCompanyHits_emptyIsError(t *testing.T) {
	_, err := parseCompanyHits("  ")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("err = %v, want empty-body failure", err)
	}
}

// TestSearch_freeFormQuery: the discovery agent's exa_search tool (WP-180)
// passes free-form queries verbatim — no category:company prefix — with
// per-query caching and budget enforcement.
func TestSearch_freeFormQuery(t *testing.T) {
	var gotQuery string
	calls := 0
	c := New("", nil)
	c.callTool = func(_ context.Context, _ string, args map[string]any) (string, error) {
		calls++
		gotQuery, _ = args["query"].(string)
		return `[{"title":"Vercel","url":"https://vercel.com"}]`, nil
	}
	hits, err := c.Search(context.Background(), "infrastructure developer tools companies", 10)
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "infrastructure developer tools companies" {
		t.Errorf("query = %q, want verbatim free-form", gotQuery)
	}
	if len(hits) != 1 || hits[0].Name != "Vercel" {
		t.Errorf("hits = %+v", hits)
	}

	// Same query again: cache hit, no new live call.
	if _, err := c.Search(context.Background(), "infrastructure developer tools companies", 10); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (cached)", calls)
	}

	// Empty query is rejected before spending.
	if _, err := c.Search(context.Background(), "  ", 10); err == nil {
		t.Error("empty query accepted")
	}
}

// TestSearch_budgetExhausted: past SetBudget, a live search errors instead
// of spending.
func TestSearch_budgetExhausted(t *testing.T) {
	c := New("", nil)
	c.callTool = func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `[{"title":"Once","url":"https://once.com"}]`, nil
	}
	c.SetBudget(1)
	if _, err := c.Search(context.Background(), "first", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(context.Background(), "second", 5); err == nil {
		t.Error("budget not enforced")
	}
}
