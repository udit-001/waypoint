package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/linkedin"
)

// keepBlockFixture: one discovered board entry (the auto-promote shape:
// Discovered flag + Domain) plus its ledger row.
type keepBlockFixture struct {
	f     *db.FakeStore
	wb    *withBoardsStub
	entry config.BoardEntry
}

func newKeepBlockFixture(t *testing.T) *keepBlockFixture {
	t.Helper()
	f := db.NewFakeStore()
	wb := &withBoardsStub{bf: &config.BoardsFile{}}
	entry := config.BoardEntry{
		Name: "vercel", Company: "Vercel", URL: "https://job-boards.greenhouse.io/vercel/",
		Provider: "greenhouse", Enabled: true, Discovered: true, Domain: "vercel.com",
	}
	wb.bf.Upsert(entry)
	_ = f.SaveCandidates([]db.CompanyCandidate{
		{Name: "Vercel", Domain: "vercel.com", Status: db.StatusCandidateAdded,
			Boards: []db.CandidateBoard{{Provider: "greenhouse", URL: entry.URL}}},
	})
	return &keepBlockFixture{f: f, wb: wb, entry: entry}
}

// TestKeepCompanyClearsFlag: keep leaves the board in place and clears
// the discovered flag — the "do nothing" action made durable.
func TestKeepCompanyClearsFlag(t *testing.T) {
	fix := newKeepBlockFixture(t)
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/vercel/keep", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("keep status = %d: %s", w.Code, w.Body.String())
	}

	e := fix.wb.bf.Find("vercel")
	if e == nil {
		t.Fatal("keep removed the board — keep keeps")
	}
	if e.Discovered {
		t.Errorf("flag still set after keep: %+v", e)
	}
}

// TestBlockCompanyRemovesAndBlocklists: block removes every board of the
// company, persists the domain on the blocklist, and tombstones the
// ledger row — never re-suggested (WP-181).
func TestBlockCompanyRemovesAndBlocklists(t *testing.T) {
	fix := newKeepBlockFixture(t)
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/vercel/block", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("block status = %d: %s", w.Code, w.Body.String())
	}

	if fix.wb.bf.Find("vercel") != nil {
		t.Error("block left the board in boards.toml")
	}
	blocked, err := fix.f.BlockedDomains()
	if err != nil || len(blocked) != 1 || blocked[0] != "vercel.com" {
		t.Errorf("BlockedDomains = %v, %v; want [vercel.com]", blocked, err)
	}
	cands, _ := fix.f.Candidates("")
	for _, c := range cands {
		if c.Name == "Vercel" && c.Status != db.StatusCandidateDismissed {
			t.Errorf("candidate status = %q, want dismissed", c.Status)
		}
	}
}

// TestBlockWithoutDomain: a manual board (no domain recorded) cannot be
// blocklisted — the request is rejected rather than half-applied.
func TestBlockWithoutDomain(t *testing.T) {
	fix := newKeepBlockFixture(t)
	fix.wb.bf.Upsert(config.BoardEntry{
		Name: "acme", Company: "Acme", URL: "https://boards.greenhouse.io/acme",
		Provider: "greenhouse", Enabled: true,
	})
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/acme/block", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("block status = %d, want 409", w.Code)
	}
	if fix.wb.bf.Find("acme") == nil {
		t.Error("rejected block still removed the board")
	}
}

// TestBlockUnknownCompany: 404, nothing written.
func TestBlockUnknownCompany(t *testing.T) {
	fix := newKeepBlockFixture(t)
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/nope/block", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("block status = %d, want 404", w.Code)
	}
}
