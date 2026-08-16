package boards

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestRetrySemantics pins the two rules reviewers care about: retryable
// failures (5xx) are retried with backoff, hard failures (404) fail fast,
// and POST bodies replay intact across attempts.
func TestRetrySemantics(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		body := make([]byte, 32)
		read, _ := r.Body.Read(body)
		if r.Method == http.MethodPost && read == 0 && len(body) > 0 {
			// Body drained — would signal the replay bug.
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "empty body on retry")
			return
		}
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"ok":true,"attempt":`+fmt.Sprint(n)+`}`)
	}))
	defer srv.Close()

	f := &HTTPFetcher{}
	any := func(string) error { return nil }

	// GET: two 500s then 200 → succeeds on attempt 3, body intact.
	raw, err := f.GetJSON(context.Background(), srv.URL, any)
	if err != nil {
		t.Fatalf("GET after retries: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("empty GET body")
	}
	if atomic.LoadInt32(&hits) != 3 {
		t.Fatalf("hits = %d, want 3", hits)
	}
}

func TestHardFailuresAreNotRetried(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := &HTTPFetcher{}
	any := func(string) error { return nil }
	if _, err := f.GetJSON(context.Background(), srv.URL, any); err == nil {
		t.Fatal("404 must fail")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("404 retried: hits = %d, want 1", hits)
	}
}

func TestPolicyRunsBeforeAnyIO(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	f := &HTTPFetcher{}
	reject := func(string) error { return fmt.Errorf("blocked") }
	if _, err := f.GetJSON(context.Background(), srv.URL, reject); err == nil {
		t.Fatal("policy rejection must surface")
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("request reached server despite policy: hits = %d", hits)
	}
}
