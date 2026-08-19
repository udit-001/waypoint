package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

func seedPostings(f *db.FakeStore) {
	f.AddPostings([]scraper.Result{
		{URL: "https://example.com/a", Title: "Engineer", Company: "Acme", Location: "Remote", Date: "2026-08-01"},
		{URL: "https://example.com/b", Title: "Designer", Company: "Beta", Location: "NYC", Date: "2026-08-05"},
	})
}

func TestListPostingsEmpty(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/postings", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var postings []db.Posting
	if err := json.Unmarshal(w.Body.Bytes(), &postings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(postings) != 0 {
		t.Errorf("expected 0 postings, got %d", len(postings))
	}
}

func TestListPostingsWithData(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/postings", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var postings []db.Posting
	if err := json.Unmarshal(w.Body.Bytes(), &postings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(postings) != 2 {
		t.Errorf("expected 2 postings, got %d", len(postings))
	}
}

func TestListPostingsFilterStatus(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	_ = f.SetPostingStatus("https://example.com/a", db.StatusDismissed)
	mux := newMuxWithLinkedIn(f, nil, nil)

	req := httptest.NewRequest("GET", "/api/postings?status=dismissed", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var postings []db.Posting
	if err := json.Unmarshal(w.Body.Bytes(), &postings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(postings) != 1 {
		t.Errorf("expected 1 dismissed posting, got %d", len(postings))
	}
	if postings[0].Result.URL != "https://example.com/a" {
		t.Errorf("expected URL a, got %s", postings[0].Result.URL)
	}
}

func TestPromotePostingSuccess(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	encoded := strings.ReplaceAll("https://example.com/a", "/", "%2F")
	req := httptest.NewRequest("POST", "/api/postings/"+encoded+"/promote", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", w.Code, w.Body.String())
	}

	var job db.Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if job.Position != "Engineer" {
		t.Errorf("position = %q, want Engineer", job.Position)
	}
	if job.Company != "Acme" {
		t.Errorf("company = %q, want Acme", job.Company)
	}

	// Posting should be marked promoted.
	p, _, _ := f.GetPosting("https://example.com/a")
	if p.Status != db.StatusPromoted {
		t.Errorf("posting status = %q, want promoted", p.Status)
	}
}

func TestPromotePostingNotFound(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	encoded := strings.ReplaceAll("https://example.com/missing", "/", "%2F")
	req := httptest.NewRequest("POST", "/api/postings/"+encoded+"/promote", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDismissPostingSuccess(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	encoded := strings.ReplaceAll("https://example.com/a", "/", "%2F")
	req := httptest.NewRequest("POST", "/api/postings/"+encoded+"/dismiss", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", w.Code, w.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["status"] != "dismissed" {
		t.Errorf("status = %q, want dismissed", result["status"])
	}

	// Posting should be marked dismissed.
	p, _, _ := f.GetPosting("https://example.com/a")
	if p.Status != db.StatusDismissed {
		t.Errorf("posting status = %q, want dismissed", p.Status)
	}
}

func TestDismissPostingNotFound(t *testing.T) {
	f := db.NewFakeStore()
	mux := newMuxWithLinkedIn(f, nil, nil)

	encoded := strings.ReplaceAll("https://example.com/missing", "/", "%2F")
	req := httptest.NewRequest("POST", "/api/postings/"+encoded+"/dismiss", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestPromotePostingIdempotent(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	mux := newMuxWithLinkedIn(f, nil, nil)

	encoded := strings.ReplaceAll("https://example.com/a", "/", "%2F")

	// First promote.
	req := httptest.NewRequest("POST", "/api/postings/"+encoded+"/promote", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("first promote: status = %d", w.Code)
	}

	// Second promote — should still 200 (idempotent).
	req = httptest.NewRequest("POST", "/api/postings/"+encoded+"/promote", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("second promote: status = %d, want 200", w.Code)
	}
}
