package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/linkedin"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/sweeper"
)

// controlsFixture: one board entry and an in-memory boards file behind
// the mutation seam.
type controlsFixture struct {
	f     *db.FakeStore
	wb    *withBoardsStub
	entry config.BoardEntry
}

func newControlsFixture(t *testing.T) *controlsFixture {
	t.Helper()
	f := db.NewFakeStore()
	wb := &withBoardsStub{bf: &config.BoardsFile{}}
	entry := config.BoardEntry{
		Name: "acme", Company: "Acme", URL: "https://boards.greenhouse.io/acme",
		Provider: "greenhouse", Enabled: true,
	}
	wb.bf.Upsert(entry)
	return &controlsFixture{f: f, wb: wb, entry: entry}
}

// stubFetch swaps the sweeper's network seam; restored on cleanup.
func stubFetch(t *testing.T, results []scraper.Result, provider string, err error) {
	t.Helper()
	orig := sweeper.FetchBoard
	sweeper.FetchBoard = func(_ context.Context, _ config.BoardEntry, _, _ int) ([]scraper.Result, string, error) {
		return results, provider, err
	}
	t.Cleanup(func() { sweeper.FetchBoard = orig })
}

func TestPauseResumeCompany(t *testing.T) {
	fix := newControlsFixture(t)
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/acme/pause", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("pause status = %d", w.Code)
	}
	if e := fix.wb.bf.Find("acme"); e == nil || e.Enabled {
		t.Errorf("entry after pause = %+v, want disabled", e)
	}

	req = httptest.NewRequest("POST", "/api/companies/acme/resume", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("resume status = %d", w.Code)
	}
	if e := fix.wb.bf.Find("acme"); e == nil || !e.Enabled {
		t.Errorf("entry after resume = %+v, want enabled", e)
	}
}

func TestRemoveAndRestoreCompany(t *testing.T) {
	fix := newControlsFixture(t)
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("DELETE", "/api/companies/acme", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 || fix.wb.bf.Find("acme") != nil {
		t.Fatalf("remove: status=%d find=%+v", w.Code, fix.wb.bf.Find("acme"))
	}

	body, _ := json.Marshal(fix.entry)
	req = httptest.NewRequest("PUT", "/api/companies/acme", strings.NewReader(string(body)))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("restore status = %d", w.Code)
	}
	if e := fix.wb.bf.Find("acme"); e == nil || !e.Enabled {
		t.Errorf("entry after restore = %+v, want back enabled", e)
	}
}

func TestRemoveUnknownCompany(t *testing.T) {
	fix := newControlsFixture(t)
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), nil, fix.wb.mutate)

	req := httptest.NewRequest("DELETE", "/api/companies/nope", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSweepNowRecordsStateAndCounts(t *testing.T) {
	fix := newControlsFixture(t)
	stubFetch(t, []scraper.Result{{URL: "https://x.com/new1", Title: "Eng", Company: "Acme"}}, "greenhouse", nil)

	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), boardsLoader(fix.entry), fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/acme/sweep", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("sweep status = %d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Meta struct {
			Fetched int  `json:"fetched"`
			New     int  `json:"new"`
			Failed  bool `json:"failed"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Meta.Failed || got.Meta.New != 1 || got.Meta.Fetched != 1 {
		t.Errorf("meta = %+v, want fetched=1 new=1 not failed", got.Meta)
	}

	// The constraint from WP-149: sweep state must be recorded, or the
	// trust strip shows stale despite fresh data.
	state, err := fix.f.GetBoardSweepStates()
	if err != nil {
		t.Fatal(err)
	}
	st, ok := state["acme"]
	if !ok || !st.OK {
		t.Errorf("sweep state = %+v, want recorded OK for acme", state)
	}
}

func TestSweepNowFailureRecordsCause(t *testing.T) {
	fix := newControlsFixture(t)
	stubFetch(t, nil, "", errors.New("rate limited"))
	mux := newMuxWithBoards(fix.f, nil, linkedin.New(), boardsLoader(fix.entry), fix.wb.mutate)

	req := httptest.NewRequest("POST", "/api/companies/acme/sweep", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("sweep status = %d (failures report in-band)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "rate limited") || !strings.Contains(w.Body.String(), `"failed":true`) {
		t.Errorf("body missing diagnosed failure: %s", w.Body.String())
	}
	state, _ := fix.f.GetBoardSweepStates()
	if st := state["acme"]; st.OK || st.Error == "" {
		t.Errorf("state = %+v, want recorded failure with cause", st)
	}

}
