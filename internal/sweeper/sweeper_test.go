package sweeper

import (
	"context"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

type fakeStore struct {
	db.FakeStore
	hasURLs map[string]bool
	jobURLs map[string]bool
	added   []string
}

func (f *fakeStore) HasPosting(url string) (bool, error) {
	return f.hasURLs[url], nil
}

func (f *fakeStore) JobExists(url string) (bool, error) {
	return f.jobURLs[url], nil
}

func (f *fakeStore) AddPostings(results []scraper.Result) error {
	for _, r := range results {
		f.added = append(f.added, r.URL)
	}
	return f.FakeStore.AddPostings(results)
}

func result(url string) scraper.Result {
	return scraper.Result{URL: url, Title: "Eng", Company: "Acme"}
}

func TestSweepOne_addsFreshAndCountsSeen(t *testing.T) {
	s := &fakeStore{
		FakeStore: *db.NewFakeStore(),
		hasURLs:   map[string]bool{"https://x.com/seen1": true},
		jobURLs:   map[string]bool{"https://x.com/tracked": true},
	}
	orig := FetchBoard
	FetchBoard = func(_ context.Context, _ config.BoardEntry, _ int, _ int) ([]scraper.Result, string, error) {
		return []scraper.Result{
			result("https://x.com/seen1"),
			result("https://x.com/tracked"),
			result("https://x.com/fresh"),
		}, "greenhouse", nil
	}
	t.Cleanup(func() { FetchBoard = orig })

	res, err := SweepOne(context.Background(), s, config.BoardEntry{Name: "acme", Company: "Acme", URL: "https://boards.greenhouse.io/acme"}, 90, 0)
	if err != nil {
		t.Fatalf("SweepOne: %v", err)
	}
	if res.Fetched != 3 || res.Seen != 2 || res.New != 1 {
		t.Errorf("result = %+v, want fetched 3 / seen 2 / new 1", res)
	}
	if len(s.added) != 1 || s.added[0] != "https://x.com/fresh" {
		t.Errorf("added = %+v, want only the fresh URL", s.added)
	}
	if res.Jobs[0].URL != "https://x.com/fresh" {
		t.Errorf("jobs = %+v, want only the fresh posting", res.Jobs)
	}
}

func TestSweepOne_providerFailureSurfaces(t *testing.T) {
	s := &fakeStore{FakeStore: *db.NewFakeStore()}
	orig := FetchBoard
	FetchBoard = func(_ context.Context, _ config.BoardEntry, _ int, _ int) ([]scraper.Result, string, error) {
		return nil, "", context.DeadlineExceeded
	}
	t.Cleanup(func() { FetchBoard = orig })

	_, err := SweepOne(context.Background(), s, config.BoardEntry{Name: "acme"}, 90, 0)
	if err == nil {
		t.Fatal("want failure surfaced")
	}
	if len(s.added) != 0 {
		t.Errorf("failed sweep wrote postings: %+v", s.added)
	}
}
