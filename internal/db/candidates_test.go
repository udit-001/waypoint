package db

import (
	"path/filepath"
	"testing"
)

func sqliteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return s.(*SQLiteStore)
}

func cand(name, domain, facet string, urls ...string) CompanyCandidate {
	var boards []CandidateBoard
	for _, u := range urls {
		boards = append(boards, CandidateBoard{URL: u})
	}
	return CompanyCandidate{Name: name, Domain: domain, Facet: facet, Boards: boards}
}

// TestSaveCandidates_insertAndDedupe: fresh candidates insert as
// suggested; a re-run with the same names does not duplicate rows.
func TestSaveCandidates_insertAndDedupe(t *testing.T) {
	s := sqliteStore(t)

	first := []CompanyCandidate{
		cand("FactSet", "factset.com", "market-data", "https://factset.wd1.myworkdayjobs.com/FactSetCareers/"),
		cand("Arcesium", "arcesium.com", "fininfra", "https://job-boards.greenhouse.io/arcesium/"),
	}
	if err := s.SaveCandidates(first); err != nil {
		t.Fatalf("SaveCandidates: %v", err)
	}

	got, err := s.Candidates("")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2", len(got))
	}
	for _, c := range got {
		if c.Status != StatusCandidateSuggested {
			t.Errorf("%s: status = %q, want suggested", c.Name, c.Status)
		}
		if c.ID == 0 {
			t.Errorf("%s: ID not assigned", c.Name)
		}
	}

	// Re-run the same batch — no duplicates.
	if err := s.SaveCandidates(first); err != nil {
		t.Fatalf("SaveCandidates re-run: %v", err)
	}
	got, err = s.Candidates("")
	if err != nil {
		t.Fatalf("Candidates after re-run: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("re-run duplicated rows: got %d, want 2", len(got))
	}
}

// TestSaveCandidates_rerunPreservesStatus: dismissed and added rows keep
// their status on re-run; a still-suggested row's boards are refreshed.
func TestSaveCandidates_rerunPreservesStatus(t *testing.T) {
	s := sqliteStore(t)

	batch1 := []CompanyCandidate{
		cand("Paytm", "paytm.com", "payments-fintech", "https://jobs.lever.co/paytm/"),
		cand("Citi", "citi.com", "banks", "https://citi.wd103.myworkdayjobs.com/CitiCareers/"),
		cand("Vanguard", "vanguard.com", "fininfra", "https://vanguard.wd1.myworkdayjobs.com/vanguard/"),
	}
	if err := s.SaveCandidates(batch1); err != nil {
		t.Fatalf("SaveCandidates: %v", err)
	}

	all, _ := s.Candidates("")
	for _, c := range all {
		switch c.Name {
		case "Paytm":
			if err := s.SetCandidateStatus(c.ID, StatusCandidateDismissed); err != nil {
				t.Fatalf("dismiss Paytm: %v", err)
			}
		case "Citi":
			if err := s.SetCandidateStatus(c.ID, StatusCandidateAdded); err != nil {
				t.Fatalf("add Citi: %v", err)
			}
		}
	}

	// Re-run: Paytm found no boards this time; Citi unchanged; Vanguard
	// gained a second board.
	batch2 := []CompanyCandidate{
		cand("Paytm", "paytm.com", "payments-fintech"),
		cand("Citi", "citi.com", "banks",
			"https://citi.wd103.myworkdayjobs.com/CitiCareers/",
			"https://jobs.eightfold.ai/careers"),
		cand("Vanguard", "vanguard.com", "fininfra",
			"https://vanguard.wd1.myworkdayjobs.com/vanguard/",
			"https://job-boards.greenhouse.io/vanguard/"),
	}
	if err := s.SaveCandidates(batch2); err != nil {
		t.Fatalf("SaveCandidates re-run: %v", err)
	}

	got, _ := s.Candidates("")
	byName := map[string]CompanyCandidate{}
	for _, c := range got {
		byName[c.Name] = c
	}
	if len(got) != 3 {
		t.Fatalf("got %d candidates, want 3 (no duplicates)", len(got))
	}
	if byName["Paytm"].Status != StatusCandidateDismissed {
		t.Errorf("Paytm status = %q, want dismissed (re-run must not reset)", byName["Paytm"].Status)
	}
	if byName["Paytm"].Domain != "paytm.com" {
		t.Errorf("Paytm domain = %q, want paytm.com (identity preserved)", byName["Paytm"].Domain)
	}
	if byName["Citi"].Status != StatusCandidateAdded {
		t.Errorf("Citi status = %q, want added (re-run must not reset)", byName["Citi"].Status)
	}
	if n := len(byName["Citi"].Boards); n != 1 {
		t.Errorf("Citi boards = %d, want 1 (added rows keep their original boards)", n)
	}
	if n := len(byName["Vanguard"].Boards); n != 2 {
		t.Errorf("Vanguard boards = %d, want 2 (suggested rows refresh boards)", n)
	}
}

// TestCandidates_statusFilter filters by status; empty means all.
func TestCandidates_statusFilter(t *testing.T) {
	s := sqliteStore(t)

	batch := []CompanyCandidate{
		cand("A", "a.com", "f1", "https://boards.greenhouse.io/a/"),
		cand("B", "b.com", "f1", "https://jobs.lever.co/b/"),
		cand("C", "c.com", "f2", "https://jobs.ashbyhq.com/c/"),
	}
	if err := s.SaveCandidates(batch); err != nil {
		t.Fatalf("SaveCandidates: %v", err)
	}
	all, _ := s.Candidates("")
	if len(all) != 3 {
		t.Fatalf("all: got %d, want 3", len(all))
	}
	if err := s.SetCandidateStatus(all[0].ID, StatusCandidateDismissed); err != nil {
		t.Fatalf("SetCandidateStatus: %v", err)
	}

	dismissed, err := s.Candidates(StatusCandidateDismissed)
	if err != nil {
		t.Fatalf("Candidates(dismissed): %v", err)
	}
	if len(dismissed) != 1 || dismissed[0].Name != "A" {
		t.Errorf("dismissed = %+v, want only A", dismissed)
	}
	suggested, err := s.Candidates(StatusCandidateSuggested)
	if err != nil {
		t.Fatalf("Candidates(suggested): %v", err)
	}
	if len(suggested) != 2 {
		t.Errorf("suggested: got %d, want 2", len(suggested))
	}
}

// TestSetCandidateStatus_unknown errors on an unknown id.
func TestSetCandidateStatus_unknown(t *testing.T) {
	s := sqliteStore(t)
	err := s.SetCandidateStatus(999, StatusCandidateAdded)
	if err == nil {
		t.Fatal("expected error for unknown candidate id")
	}
}
