package boards

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// HostPolicy validates a URL before a single network request is made.
// Each provider supplies its own policy (host allowlist) so an untrusted
// server-side redirect can never bounce a fetch off the allowed host.
type HostPolicy func(rawURL string) error

// JSONFetcher retrieves JSON documents. Providers depend on this seam for
// testability: tests inject a fake; production uses HTTPFetcher.
type JSONFetcher interface {
	GetJSON(ctx context.Context, rawURL string, policy HostPolicy) (json.RawMessage, error)
	PostJSON(ctx context.Context, rawURL string, body []byte,
		headers map[string]string, policy HostPolicy) (json.RawMessage, error)
}

// HTTPFetcher is the default JSONFetcher adapter: browser-like User-Agent,
// capped exponential backoff on retryable failures (429/5xx and network
// errors), a redirect guard so redirects cannot escape the host policy,
// and a response-size cap.
type HTTPFetcher struct {
	// UserAgent overrides the default browser-like UA.
	UserAgent string
	// MaxRespBytes caps each response body. Defaults to 10 MiB.
	MaxRespBytes int64
}

const defaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const defaultMaxRespBytes = 10 << 20 // 10 MiB

// retryableError marks a failure worth retrying (429/5xx, network errors).
// Hard failures (404, bad URL) fail fast instead of burning backoff.
type retryableError struct{ err error }

func (e retryableError) Error() string { return e.err.Error() }

// client returns an http.Client whose redirect handler re-applies the host
// policy at every hop, so a server-side redirect can never escape the
// provider's allowlist (SSRF guard).
func (f *HTTPFetcher) client(policy HostPolicy) *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return policy(req.URL.String())
		},
	}
}

// doRetry executes a request with capped exponential backoff on retryable
// failures. Request bodies replay correctly on retry because requests are
// built with *bytes.Reader, which sets http.Request.GetBody.
func (f *HTTPFetcher) doRetry(client *http.Client, ctx context.Context, req *http.Request) (json.RawMessage, error) {
	const maxRetries = 3
	delay := 500 * time.Millisecond

	for attempt := 0; ; attempt++ {
		body, err := f.doOnce(client, req)
		if err == nil {
			return json.RawMessage(body), nil
		}
		if _, ok := err.(retryableError); !ok || attempt == maxRetries {
			return nil, err
		}
		select {
		case <-time.After(delay + jitter()):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		delay *= 2
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
	}
}

// doOnce performs a single request attempt.
func (f *HTTPFetcher) doOnce(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, retryableError{err} // network errors: retry
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, retryableError{fmt.Errorf("request failed: %d %s", resp.StatusCode, resp.Status)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %d %s", resp.StatusCode, resp.Status)
	}
	limit := f.MaxRespBytes
	if limit <= 0 {
		limit = defaultMaxRespBytes
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("response too large: %d bytes", resp.ContentLength)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// makeReq builds a request and applies the default headers.
func makeReq(ctx context.Context, method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUA)
	}
	return req, nil
}

// GetJSON fetches raw JSON from rawURL. The policy runs before any I/O and
// again on every redirect hop (see client).
func (f *HTTPFetcher) GetJSON(ctx context.Context, rawURL string, policy HostPolicy) (json.RawMessage, error) {
	if err := policy(rawURL); err != nil {
		return nil, err
	}
	req, err := makeReq(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if f.UserAgent != "" {
		req.Header.Set("User-Agent", f.UserAgent)
	}
	return f.doRetry(f.client(policy), ctx, req)
}

// PostJSON performs a JSON POST (used by Workday's CXS endpoint). The
// policy runs before the request and on every redirect hop.
func (f *HTTPFetcher) PostJSON(ctx context.Context, rawURL string, body []byte,
	headers map[string]string, policy HostPolicy) (json.RawMessage, error) {
	if err := policy(rawURL); err != nil {
		return nil, err
	}
	req, err := makeReq(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if f.UserAgent != "" {
		req.Header.Set("User-Agent", f.UserAgent)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return f.doRetry(f.client(policy), ctx, req)
}

// jitter returns a small randomized delay to avoid thundering herd on 429.
func jitter() time.Duration {
	return time.Duration(rand.Intn(200)) * time.Millisecond
}
