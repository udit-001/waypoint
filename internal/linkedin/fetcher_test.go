package linkedin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchProfileCallsWebFetchExa(t *testing.T) {
	var gotTool string
	var gotArgs map[string]any
	f := New(WithCallTool(func(_ context.Context, tool string, args map[string]any) (string, error) {
		gotTool = tool
		gotArgs = args
		return fixtureMarkdown, nil
	}))

	p, err := f.FetchProfile(context.Background(), "https://www.linkedin.com/in/janedoe")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if gotTool != "web_fetch_exa" {
		t.Errorf("tool = %q, want web_fetch_exa", gotTool)
	}
	urls, _ := gotArgs["urls"].([]string)
	if len(urls) != 1 || urls[0] != "https://www.linkedin.com/in/janedoe" {
		t.Errorf("args[urls] = %v", gotArgs["urls"])
	}
	if p.Name != "Jane Doe" {
		t.Errorf("parsed Name = %q, want Jane Doe", p.Name)
	}
}

func TestFetchProfilePropagatesToolError(t *testing.T) {
	f := New(WithCallTool(func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "", errors.New("rate limited")
	}))
	_, err := f.FetchProfile(context.Background(), "https://www.linkedin.com/in/janedoe")
	if err == nil || !strings.Contains(err.Error(), "exa fetch") {
		t.Errorf("err = %v, want exa fetch wrapped error", err)
	}
}

func TestFetchProfileRejectsNonLinkedInURL(t *testing.T) {
	f := New(WithCallTool(func(_ context.Context, _ string, _ map[string]any) (string, error) {
		t.Fatal("callTool must not run for an invalid URL")
		return "", nil
	}))
	for _, u := range []string{
		"https://example.com/in/janedoe",
		"https://www.linkedin.com/company/acme",
		"not a url",
		"",
		"http://notlinkedin.com/in/x",
	} {
		if _, err := f.FetchProfile(context.Background(), u); err == nil {
			t.Errorf("FetchProfile(%q) should fail validation", u)
		}
	}
}

func TestValidateURL(t *testing.T) {
	valid := []string{
		"https://www.linkedin.com/in/janedoe",
		"https://in.linkedin.com/in/janedoe",
		"https://uk.linkedin.com/in/janedoe",
		"http://www.linkedin.com/in/janedoe?trk=blah",
		"https://linkedin.com/in/janedoe",
	}
	for _, u := range valid {
		if _, err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want nil", u, err)
		}
	}
	invalid := []string{
		"",
		"https://example.com/in/janedoe",
		"https://www.linkedin.com/company/acme",
		"https://www.linkedin.com/jobs/view/123",
		"ftp://www.linkedin.com/in/janedoe",
		"https://notlinkedin.com/in/janedoe",
	}
	for _, u := range invalid {
		if _, err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) should fail", u)
		}
	}
}

// fakeExaMCP serves just enough MCP for callExaTool: initialize (issues a
// session id) and tools/call (returns fixture markdown). It records the
// Authorization header of each phase so tests can assert credential flow
// over the real client path — no callTool stubbing.
type fakeExaMCP struct {
	srv          *httptest.Server
	initAuth     string
	callAuth     string
	callSessions map[string]string // request # → session header
}

func newFakeExaMCP(t *testing.T) *fakeExaMCP {
	t.Helper()
	fx := &fakeExaMCP{callSessions: map[string]string{}}
	fx.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			fx.initAuth = r.Header.Get("Authorization")
			w.Header().Set("Mcp-Session-Id", "ses-fake")
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`)
		case "tools/call":
			fx.callAuth = r.Header.Get("Authorization")
			fx.callSessions["tool"] = r.Header.Get("Mcp-Session-Id")
			io.WriteString(w, `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"# Jane Doe\nTitle: Senior Product Designer"}]}}`)
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	t.Cleanup(fx.srv.Close)
	return fx
}

// The saved-key contract, verified at the package's real seam: whatever
// WithAPIKeyFunc resolves becomes a Bearer header on both MCP phases, the
// session id still rides along, and rotation between fetches is honored
// (the resolver runs per fetch — Settings can change under a live server).
func TestFetchProfileSendsResolvedAPIKey(t *testing.T) {
	savedKey := "exa-key-first"
	fx := newFakeExaMCP(t)
	f := New(
		WithEndpoint(fx.srv.URL),
		WithAPIKeyFunc(func() string { return savedKey }),
	)

	if _, err := f.FetchProfile(context.Background(), "https://www.linkedin.com/in/janedoe"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	want := "Bearer exa-key-first"
	if fx.initAuth != want || fx.callAuth != want {
		t.Errorf("auth headers = init %q / call %q, want %q on both", fx.initAuth, fx.callAuth, want)
	}
	if fx.callSessions["tool"] != "ses-fake" {
		t.Errorf("tools/call Mcp-Session-Id = %q, want ses-fake", fx.callSessions["tool"])
	}

	// Key rotated in settings between fetches → next fetch carries it.
	savedKey = "exa-key-second"
	if _, err := f.FetchProfile(context.Background(), "https://www.linkedin.com/in/janedoe"); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got := "Bearer exa-key-second"; fx.callAuth != got {
		t.Errorf("rotated auth = %q, want %q", fx.callAuth, got)
	}
}

// Empty resolution means anonymous — no half credentials on the wire
// (mirrors internal/exa: an invalid Bearer is worse than none).
func TestFetchProfileAnonymousWhenResolverEmpty(t *testing.T) {
	fx := newFakeExaMCP(t)
	f := New(WithEndpoint(fx.srv.URL), WithAPIKeyFunc(func() string { return "   " }))

	if _, err := f.FetchProfile(context.Background(), "https://www.linkedin.com/in/janedoe"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if fx.initAuth != "" || fx.callAuth != "" {
		t.Errorf("blank key leaked auth headers: init %q / call %q", fx.initAuth, fx.callAuth)
	}
}
