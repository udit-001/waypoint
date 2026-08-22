package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/linkedin"
	"github.com/udit-001/waypoint/internal/scraper"
)

func boardsLoader(entries ...config.BoardEntry) func() ([]config.BoardEntry, error) {
	return func() ([]config.BoardEntry, error) { return entries, nil }
}

// TestListCompanies_empty: no boards.toml entries (or no boards file at all)
// yields an empty companies array — the empty state's raw material.
func TestListCompanies_empty(t *testing.T) {
	mux := newMuxWithBoards(db.NewFakeStore(), nil, linkedin.New(), boardsLoader(), nil)

	req := httptest.NewRequest("GET", "/api/companies", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got struct {
		Companies []map[string]any `json:"companies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Companies == nil || len(got.Companies) != 0 {
		t.Errorf("companies = %v, want non-nil empty array", got.Companies)
	}
}

// TestListCompanies_mergeSort: rows merge boards.toml with live counts and
// sweep state; companies with new postings float above the rest regardless
// of name order, the rest sort alphabetically.
func TestListCompanies_mergeSort(t *testing.T) {
	f := db.NewFakeStore()
	f.AddPostings([]scraper.Result{
		{URL: "https://x.com/1", Title: "Eng", Company: "Beta"},
	})
	states := map[string]db.BoardSweepState{
		"beta": {At: time.Now().UTC().Format(time.RFC3339), OK: true},
	}
	f.SetBoardSweepState("beta", states["beta"])
	f.SetBoardSweepState("acme", db.BoardSweepState{At: time.Now().UTC().Format(time.RFC3339), OK: true})

	boards := []config.BoardEntry{
		{Name: "acme", Company: "Acme", URL: "https://acme.com/jobs", Provider: "greenhouse", Enabled: true},
		{Name: "beta", Company: "Beta", URL: "https://boards.greenhouse.io/beta", Provider: "greenhouse", Enabled: true},
	}
	mux := newMuxWithBoards(f, nil, linkedin.New(), boardsLoader(boards...), nil)

	req := httptest.NewRequest("GET", "/api/companies", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got struct {
		Companies []CompanyView `json:"companies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got.Companies) != 2 {
		t.Fatalf("companies = %d, want 2", len(got.Companies))
	}

	first, second := got.Companies[0], got.Companies[1]
	if first.Name != "beta" || first.NewCount != 1 {
		t.Errorf("first row = %+v, want beta with newCount 1 floating on top", first)
	}
	if second.Name != "acme" || second.NewCount != 0 {
		t.Errorf("second row = %+v, want acme with newCount 0 below", second)
	}
	if first.Provider != "greenhouse" || !first.Enabled {
		t.Errorf("beta row missing provider chip / enabled: %+v", first)
	}
	if first.LastSweptAt == "" || !first.LastSweepOk {
		t.Errorf("beta row missing trust strip: %+v", first)
	}
}

// TestListCompanies_staleAndError: a sweep older than two cycles flags
// stale; a failed sweep carries its inline diagnostic.
func TestListCompanies_staleAndError(t *testing.T) {
	f := db.NewFakeStore()
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	f.SetBoardSweepState("acme", db.BoardSweepState{At: old, OK: true})
	f.SetBoardSweepState("beta", db.BoardSweepState{
		At: time.Now().UTC().Format(time.RFC3339), OK: false, Error: "rate-limited",
	})

	boards := []config.BoardEntry{
		{Name: "acme", Company: "Acme", URL: "https://acme.com/jobs", Provider: "greenhouse", Enabled: true},
		{Name: "beta", Company: "Beta", URL: "https://beta.com/jobs", Provider: "lever", Enabled: true},
	}
	mux := newMuxWithBoards(f, nil, linkedin.New(), boardsLoader(boards...), nil)

	req := httptest.NewRequest("GET", "/api/companies", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got struct {
		Companies []CompanyView `json:"companies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	byName := map[string]CompanyView{}
	for _, c := range got.Companies {
		byName[c.Name] = c
	}
	acme, ok := byName["acme"]
	if !ok {
		t.Fatal("acme row missing")
	}
	if !acme.Stale {
		t.Errorf("acme (swept 48h ago, 6h cadence) not flagged stale: %+v", acme)
	}
	beta := byName["beta"]
	if beta.LastSweepOk || beta.LastSweepError != "rate-limited" {
		t.Errorf("beta error diagnostic missing: %+v", beta)
	}
	if beta.Stale {
		t.Errorf("beta swept just now should not be stale: %+v", beta)
	}
}
