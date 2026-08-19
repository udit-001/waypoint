package mdsvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeFetch records every URL requested and answers from a routing
// function. It stands in for HTTP at the package seam.
type fakeFetch struct {
	calls []string
	route func(url string) *http.Response
}

func (f *fakeFetch) Fetch(ctx context.Context, url string) (*http.Response, error) {
	f.calls = append(f.calls, url)
	return f.route(url), nil
}

// resp builds an *http.Response with a string body.
func resp(status int, header map[string]string, body string) *http.Response {
	h := http.Header{}
	for k, v := range header {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// longBody makes content past the minimum-length gate.
func longBody(prefix string) string {
	return prefix + " " + strings.Repeat("lorem ipsum dolor sit amet ", 8)
}

const greenhouseURL = "https://job-boards.greenhouse.io/algolia/jobs/6006191004"

func mdnewBody() string {
	return "Title: Customer Success Manager\n\nURL Source: " + greenhouseURL +
		"\n\nMarkdown Content:\n---\ntitle: Customer Success Manager\n---\n\n# Customer Success Manager\n\n" +
		longBody("At Algolia, we power search.")
}

func compressBody() string {
	return "# Customer Success Manager\n\n" + longBody("At Algolia, we power search.")
}

func isMarkdownNew(url string) bool { return strings.HasPrefix(url, "https://markdown.new/") }

// readState loads the persisted state file for assertions.
func readState(t *testing.T, path string) stateFile {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var s stateFile
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	return s
}

func TestFetchPrimaryServiceWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	ff := &fakeFetch{route: func(url string) *http.Response {
		if isMarkdownNew(url) {
			return resp(200, nil, mdnewBody())
		}
		return resp(200, nil, compressBody())
	}}
	c := New(path)
	c.fetch = ff.Fetch

	got, err := c.Fetch(context.Background(), greenhouseURL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(got, "# Customer Success Manager") || strings.Contains(got, "Markdown Content:") {
		t.Errorf("preamble/frontmatter not stripped: %q", got[:min(80, len(got))])
	}
	if !strings.Contains(got, "At Algolia") {
		t.Errorf("body content missing")
	}
	if len(ff.calls) != 1 || !isMarkdownNew(ff.calls[0]) {
		t.Errorf("expected exactly markdown.new call, got %v", ff.calls)
	}
	s := readState(t, path)
	if s.Services["markdown.new"].Used != 1 {
		t.Errorf("markdown.new used = %d, want 1", s.Services["markdown.new"].Used)
	}
}

func TestFetchFailoverOn429(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	ff := &fakeFetch{route: func(url string) *http.Response {
		if isMarkdownNew(url) {
			return resp(429, map[string]string{"Retry-After": "3600"}, "")
		}
		return resp(200, nil, compressBody())
	}}
	c := New(path)
	c.fetch = ff.Fetch

	got, err := c.Fetch(context.Background(), greenhouseURL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(got, "At Algolia") {
		t.Errorf("expected compress.new content")
	}
	if len(ff.calls) != 2 {
		t.Errorf("expected markdown.new then compress.new, got %v", ff.calls)
	}
	s := readState(t, path)
	if !s.Services["markdown.new"].Exhausted {
		t.Errorf("markdown.new not marked exhausted after 429")
	}
	if s.Services["compress.new"].Used != 1 {
		t.Errorf("compress.new used = %d, want 1", s.Services["compress.new"].Used)
	}
}

func TestFetchSkipsServiceAtDailyQuota(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	start := stateFile{
		Day: today(),
		Services: map[string]*svcUse{
			"markdown.new": {Used: DailyQuota, Remaining: -1},
		},
	}
	b, _ := json.Marshal(start)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	ff := &fakeFetch{route: func(url string) *http.Response {
		return resp(200, nil, compressBody())
	}}
	c := New(path)
	c.fetch = ff.Fetch

	if _, err := c.Fetch(context.Background(), greenhouseURL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	for _, url := range ff.calls {
		if isMarkdownNew(url) {
			t.Errorf("markdown.new called despite spent quota: %v", ff.calls)
		}
	}
}

func TestFetchAllExhausted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	start := stateFile{
		Day: today(),
		Services: map[string]*svcUse{
			"markdown.new": {Used: 3, Exhausted: true},
			"compress.new": {Used: 5, Exhausted: true},
		},
	}
	b, _ := json.Marshal(start)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	ff := &fakeFetch{route: func(url string) *http.Response {
		return resp(200, nil, mdnewBody())
	}}
	c := New(path)
	c.fetch = ff.Fetch

	if _, err := c.Fetch(context.Background(), greenhouseURL); err == nil {
		t.Fatal("expected error when all services exhausted")
	} else if !strings.Contains(err.Error(), "quota") {
		t.Errorf("error should mention quota, got: %v", err)
	}
	if len(ff.calls) != 0 {
		t.Errorf("no HTTP calls expected, got %v", ff.calls)
	}
}

func TestFetchDayRolloverResets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	stale := stateFile{
		Day: "2000-01-01", // any day that is not today
		Services: map[string]*svcUse{
			"markdown.new": {Used: DailyQuota, Exhausted: true},
			"compress.new": {Used: DailyQuota, Exhausted: true},
		},
	}
	b, _ := json.Marshal(stale)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	ff := &fakeFetch{route: func(url string) *http.Response {
		return resp(200, nil, mdnewBody())
	}}
	c := New(path)
	c.fetch = ff.Fetch

	if _, err := c.Fetch(context.Background(), greenhouseURL); err != nil {
		t.Fatalf("stale-day state must reset: %v", err)
	}
	s := readState(t, path)
	if s.Day != today() || s.Services["markdown.new"].Used != 1 {
		t.Errorf("state not rolled over: %+v", s)
	}
}

func TestFetchRejectsShortContent(t *testing.T) {
	ff := &fakeFetch{route: func(url string) *http.Response {
		return resp(200, nil, "# too short")
	}}
	c := New("")
	c.fetch = ff.Fetch

	if _, err := c.Fetch(context.Background(), greenhouseURL); err == nil {
		t.Fatal("expected error for under-length content")
	}
	if len(ff.calls) != 2 {
		t.Errorf("expected failover to second service, got %v", ff.calls)
	}
}

func TestRemainingZeroMarksExhausted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	ff := &fakeFetch{route: func(url string) *http.Response {
		if isMarkdownNew(url) {
			return resp(200, map[string]string{"x-rate-limit-remaining": "0"}, mdnewBody())
		}
		return resp(200, nil, compressBody())
	}}
	c := New(path)
	c.fetch = ff.Fetch

	if _, err := c.Fetch(context.Background(), greenhouseURL); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if len(ff.calls) != 1 {
		t.Fatalf("first Fetch should hit only markdown.new, got %v", ff.calls)
	}
	if _, err := c.Fetch(context.Background(), greenhouseURL); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if isMarkdownNew(ff.calls[1]) {
		t.Errorf("markdown.new reused despite remaining=0: %v", ff.calls)
	}
	s := readState(t, path)
	if !s.Services["markdown.new"].Exhausted {
		t.Errorf("markdown.new not marked exhausted at remaining=0")
	}
}

func TestClean(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			"markdown.new preamble + frontmatter",
			"Title: T\n\nURL Source: https://x\n\nMarkdown Content:\n---\ntitle: T\n---\n\nBody here.",
			"Body here.",
		},
		{
			"frontmatter only",
			"---\ntitle: T\n---\nBody here.",
			"Body here.",
		},
		{
			"compress.new passthrough",
			"# Title\n\nBody here.",
			"# Title\n\nBody here.",
		},
		{
			"empty",
			"   \n",
			"",
		},
	}
	for _, tc := range cases {
		if got := Clean(tc.in); got != tc.want {
			t.Errorf("%s: Clean = %q, want %q", tc.name, got, tc.want)
		}
	}
}
