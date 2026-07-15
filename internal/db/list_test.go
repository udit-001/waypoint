package db

import (
	"testing"
	"time"
)

// seedDeadlineJobs populates a FakeStore with jobs at various deadline offsets
// relative to today. Returns the store and a map of label -> job ID for assertions.
func seedDeadlineJobs(t *testing.T) *FakeStore {
	t.Helper()
	now := time.Now()
	dateStr := func(offset int) string {
		return now.AddDate(0, 0, offset).Format("2006-01-02")
	}

	f := NewFakeStore()
	jobs := []struct {
		id    int64
		date  string
		label string
	}{
		{1, dateStr(-30), "past"},
		{2, dateStr(-1), "yesterday"},
		{3, dateStr(0), "today"},
		{4, dateStr(3), "in3days"},
		{5, dateStr(7), "in7days"},
		{6, dateStr(30), "in30days"},
		{7, "", "nodeadline"},
	}
	for _, j := range jobs {
		f.Jobs[j.id] = Job{
			ID:       j.id,
			Company:  j.label,
			Position: "SWE",
			Status:   "Wishlist",
			Date:     j.date,
		}
	}
	return f
}

func jobLabels(jobs []Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Company
	}
	return out
}

func TestListJobs_noFilters(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	if len(jobs) != 7 {
		t.Fatalf("expected 7 jobs, got %d", len(jobs))
	}
}

func TestListJobs_expired(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{Expired: true})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	got := jobLabels(jobs)
	want := []string{"yesterday", "past"}
	if len(got) != len(want) {
		t.Fatalf("expected %d expired jobs (%v), got %d (%v)", len(want), want, len(got), got)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected expired job %q in results, got %v", w, got)
		}
	}
}

func TestListJobs_expiringSoon(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{ExpiringSoon: true})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	got := jobLabels(jobs)
	want := []string{"today", "in3days", "in7days"}
	if len(got) != len(want) {
		t.Fatalf("expected %d expiring-soon jobs (%v), got %d (%v)", len(want), want, len(got), got)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected expiring-soon job %q in results, got %v", w, got)
		}
	}
}

func TestListJobs_expiredAndExpiringSoon(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{Expired: true, ExpiringSoon: true})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	got := jobLabels(jobs)
	// Union of expired + expiring-soon, excluding no-deadline and far-future.
	want := []string{"past", "yesterday", "today", "in3days", "in7days"}
	if len(got) != len(want) {
		t.Fatalf("expected %d jobs (%v), got %d (%v)", len(want), want, len(got), got)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected job %q in combined results, got %v", w, got)
		}
	}
}

func TestListJobs_expiredExcludesNoDeadline(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{Expired: true})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	for _, j := range jobs {
		if j.Date == "" {
			t.Error("job with no deadline should not appear in --expired results")
		}
	}
}

func TestListJobs_expiringSoonExcludesNoDeadline(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{ExpiringSoon: true})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	for _, j := range jobs {
		if j.Date == "" {
			t.Error("job with no deadline should not appear in --expiring-soon results")
		}
	}
}

func TestListJobs_expiredComposesWithStatus(t *testing.T) {
	f := seedDeadlineJobs(t)
	// Add an expired job with a different status.
	f.Jobs[8] = Job{
		ID:      8,
		Company: "expired-applied",
		Date:    time.Now().AddDate(0, 0, -5).Format("2006-01-02"),
		Status:  "Applied",
	}
	jobs, err := ListJobs(f, ListOpts{Expired: true, Status: "Wishlist"})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	for _, j := range jobs {
		if j.Status != "Wishlist" {
			t.Errorf("expected only Wishlist jobs, got status %q for %q", j.Status, j.Company)
		}
	}
}

func TestListJobs_expiringSoonComposesWithSearch(t *testing.T) {
	f := seedDeadlineJobs(t)
	jobs, err := ListJobs(f, ListOpts{ExpiringSoon: true, Search: "today"})
	if err != nil {
		t.Fatalf("ListJobs error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job matching 'today', got %d (%v)", len(jobs), jobLabels(jobs))
	}
	if jobs[0].Company != "today" {
		t.Errorf("expected 'today' job, got %q", jobs[0].Company)
	}
}

func TestListJobs_expiringSoonDaysConstant(t *testing.T) {
	if expiringSoonDays != 7 {
		t.Errorf("expected expiringSoonDays=7, got %d", expiringSoonDays)
	}
}
