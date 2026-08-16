package linkedin

import (
	"context"
	"errors"
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
