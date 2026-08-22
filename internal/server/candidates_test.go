package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/linkedin"
)

// seedCandidate stores one suggested Acme candidate in the fake store.
func seedCandidate(t *testing.T, f *db.FakeStore) db.CompanyCandidate {
	t.Helper()
	cand := db.CompanyCandidate{
		Name:   "Acme Corp",
		Domain: "acme.com",
		Facet:  "market-data",
		Boards: []db.CandidateBoard{{Provider: "greenhouse", URL: "https://boards.greenhouse.io/acme"}},
	}
	if err := f.SaveCandidates([]db.CompanyCandidate{cand}); err != nil {
		t.Fatal(err)
	}
	got, _ := f.Candidates("")
	cand.ID = got[0].ID
	return cand
}

// withBoardsStub backs the mutation seam with an in-memory file and
// records whether a save happened.
type withBoardsStub struct {
	bf    *config.BoardsFile
	saved bool
}

func (w *withBoardsStub) mutate(fn func(*config.BoardsFile) error) error {
	if err := fn(w.bf); err != nil {
		return err
	}
	w.saved = true
	return nil
}

// stubServerProbe swaps the promotion network seam; restored on cleanup.
func stubServerProbe(t *testing.T, jobs int, err error) {
	t.Helper()
	orig := discovery.ProbeBoard
	discovery.ProbeBoard = func(_ context.Context, _ discovery.BoardLink) (int, error) {
		return jobs, err
	}
	t.Cleanup(func() { discovery.ProbeBoard = orig })
}

func postCandidate(t *testing.T, mux http.Handler, id int64, action string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/candidates/"+strconv.FormatInt(id, 10)+"/"+action, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestListCandidates(t *testing.T) {
	f := db.NewFakeStore()
	seedCandidate(t, f)
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, nil)

	req := httptest.NewRequest("GET", "/api/candidates", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got struct {
		Candidates []map[string]any `json:"candidates"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0]["name"] != "Acme Corp" {
		t.Errorf("candidates = %+v, want Acme Corp", got.Candidates)
	}

	req = httptest.NewRequest("GET", "/api/candidates?status=dismissed", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got.Candidates) != 0 {
		t.Errorf("dismissed filter returned %+v, want empty", got.Candidates)
	}
}

func TestDismissCandidate(t *testing.T) {
	f := db.NewFakeStore()
	cand := seedCandidate(t, f)
	wb := &withBoardsStub{bf: &config.BoardsFile{}}
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, wb.mutate)

	w := postCandidate(t, mux, cand.ID, "dismiss")
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"dismissed"`) || !strings.Contains(w.Body.String(), `"updated":true`) {
		t.Errorf("body missing dismissed/updated: %s", w.Body.String())
	}

	if got, _ := f.Candidates(db.StatusCandidateDismissed); len(got) != 1 || got[0].Name != "Acme Corp" {
		t.Errorf("dismissed candidates = %+v, want Acme Corp", got)
	}
	events, _ := f.ChangesSince(0, 10)
	if len(events) == 0 || events[len(events)-1].Kind != "candidates" {
		t.Errorf("change events = %+v, want a candidates event", events)
	}
}

func TestAddCandidatePromotesAndFlipsStatus(t *testing.T) {
	f := db.NewFakeStore()
	cand := seedCandidate(t, f)
	stubServerProbe(t, 4, nil)
	wb := &withBoardsStub{bf: &config.BoardsFile{}}
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, wb.mutate)

	w := postCandidate(t, mux, cand.ID, "add")
	if w.Code != 200 {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !wb.saved {
		t.Error("boards file never saved")
	}
	e := wb.bf.Find("acme-corp")
	if e == nil || !e.Enabled || e.Provider != "greenhouse" {
		t.Errorf("boards entry = %+v, want enabled greenhouse acme-corp", e)
	}
	if got, _ := f.Candidates(db.StatusCandidateAdded); len(got) != 1 {
		t.Errorf("candidate status = suggested, want added (%+v)", got)
	}
}

func TestAddCandidateAlreadyWatchedIsNoOp(t *testing.T) {
	f := db.NewFakeStore()
	cand := seedCandidate(t, f)
	stubServerProbe(t, 4, nil)
	wb := &withBoardsStub{bf: &config.BoardsFile{}}
	wb.bf.Upsert(config.BoardEntry{Name: "acme-manual", Company: "Acme", URL: cand.Boards[0].URL, Enabled: true})
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, wb.mutate)

	w := postCandidate(t, mux, cand.ID, "add")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already in boards.toml") {
		t.Errorf("body missing no-op message: %s", w.Body.String())
	}
	if got, _ := f.Candidates(db.StatusCandidateSuggested); len(got) != 1 {
		t.Errorf("no-op must not decide; candidates = %+v", got)
	}
}

func TestAddCandidateVerifyFailureSurfacesCause(t *testing.T) {
	f := db.NewFakeStore()
	cand := seedCandidate(t, f)
	stubServerProbe(t, 0, errors.New("connection refused"))
	wb := &withBoardsStub{bf: &config.BoardsFile{}}
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, wb.mutate)

	w := postCandidate(t, mux, cand.ID, "add")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "verification failed") {
		t.Errorf("body missing cause: %s", w.Body.String())
	}
	if got, _ := f.Candidates(db.StatusCandidateSuggested); len(got) != 1 {
		t.Errorf("failed verify must not decide; candidates = %+v", got)
	}
}

func TestAddWithoutBoardsWriterFails(t *testing.T) {
	f := db.NewFakeStore()
	cand := seedCandidate(t, f)
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, nil)

	w := postCandidate(t, mux, cand.ID, "add")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
}

func TestCandidateUnknownID(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithBoards(f, nil, linkedin.New(), nil, nil)

	for _, action := range []string{"add", "dismiss"} {
		w := postCandidate(t, mux, 999, action)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s unknown id: status = %d, want 404", action, w.Code)
		}
	}
	w := postCandidate(t, mux, -7, "add")
	if w.Code != http.StatusBadRequest {
		t.Errorf("negative id: status = %d, want 400", w.Code)
	}
}
