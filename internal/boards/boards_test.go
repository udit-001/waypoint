package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// fakeFetcher serves canned JSON by URL prefix, recording requested URLs.
type fakeFetcher struct {
	responses map[string]string // URL prefix → JSON body
	requested []string
}

func (f *fakeFetcher) GetJSON(ctx context.Context, rawURL string, policy HostPolicy) (json.RawMessage, error) {
	if err := policy(rawURL); err != nil {
		return nil, err
	}
	f.requested = append(f.requested, rawURL)
	for prefix, body := range f.responses {
		if strings.HasPrefix(rawURL, prefix) {
			return json.RawMessage(body), nil
		}
	}
	return nil, fmt.Errorf("fake: no response for %s", rawURL)
}

func (f *fakeFetcher) PostJSON(ctx context.Context, rawURL string, body []byte,
	headers map[string]string, policy HostPolicy) (json.RawMessage, error) {
	return f.GetJSON(ctx, rawURL, policy)
}

const greenhouseFixture = `{"jobs":[
  {"id":123,"title":"Software Engineer","absolute_url":"https://job-boards.greenhouse.io/khanacademy/jobs/1","location":{"name":"Remote"},"updated_at":"2026-08-01T00:00:00Z"},
  {"id":124,"title":"Curriculum Designer","absolute_url":"https://job-boards.greenhouse.io/khanacademy/jobs/2","location":{"name":"Mountain View, CA"},"updated_at":"2026-08-10T00:00:00Z"},
  {"id":125,"title":"","absolute_url":"https://job-boards.greenhouse.io/khanacademy/jobs/3","location":{"name":"X"},"updated_at":"2026-08-10T00:00:00Z"}
]}`

func TestGreenhouseDetectAndFetch(t *testing.T) {
	g := Greenhouse{Fetcher: &fakeFetcher{responses: map[string]string{
		"https://boards-api.greenhouse.io/v1/boards/khanacademy/jobs": greenhouseFixture,
	}}}

	b := Board{Name: "khanacademy", Company: "Khan Academy", URL: "https://job-boards.greenhouse.io/khanacademy/"}
	p, hit, err := DetectProvider(b)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if p.Name() != "greenhouse" {
		t.Fatalf("provider = %s, want greenhouse", p.Name())
	}
	if hit.API != "https://boards-api.greenhouse.io/v1/boards/khanacademy/jobs" {
		t.Fatalf("api = %s", hit.API)
	}

	results, err := g.Fetch(context.Background(), b, *hit, FetchOpts{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(results) != 2 { // blank-title row dropped
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].ID != "123" {
		t.Fatalf("numeric id not stringified: %q", results[0].ID)
	}
	if results[0].Date != "2026-08-01" {
		t.Fatalf("date = %q", results[0].Date)
	}
	if results[0].Company != "Khan Academy" {
		t.Fatalf("company = %q", results[0].Company)
	}
}

func TestGreenhouseRejectsForeignHost(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{
		"https://boards-api.greenhouse.io": greenhouseFixture,
	}}
	g := Greenhouse{Fetcher: f}
	// A board on an attacker host: Detect must not claim it.
	b := Board{Name: "evil", URL: "https://evil.example.com/khanacademy/"}
	if hit, _ := g.Detect(b); hit != nil {
		t.Fatal("detect claimed foreign host")
	}
	// And the policy must reject any direct API hit on a foreign host.
	if _, err := f.GetJSON(context.Background(),
		"https://evil.example.com/v1/boards/khanacademy/jobs", g.policy()); err == nil {
		t.Fatal("policy allowed foreign host")
	}
}

const leverFixture = `[{"id":"aaa-111","text":"Backend Engineer","hostedUrl":"https://jobs.lever.co/3pillarglobal/aaa-111","createdAt":1754000000000,"categories":{"location":"Remote"},"descriptionPlain":"Build things."}]`

func TestLeverDetectAndFetch(t *testing.T) {
	l := Lever{Fetcher: &fakeFetcher{responses: map[string]string{
		"https://api.lever.co/v0/postings/3pillarglobal": leverFixture,
	}}}
	b := Board{Name: "3pillarglobal", Company: "3Pillar Global", URL: "https://jobs.lever.co/3pillarglobal"}
	p, hit, err := DetectProvider(b)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if p.Name() != "lever" || hit.API != "https://api.lever.co/v0/postings/3pillarglobal" {
		t.Fatalf("provider=%s api=%s", p.Name(), hit.API)
	}
	results, err := l.Fetch(context.Background(), b, *hit, FetchOpts{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(results) != 1 || results[0].Description != "Build things." || results[0].Date == "" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

const bambooListFixture = `{"result":[{"id":"42","jobOpeningName":"Woodworker","location":{"city":"Morrisville","state":"VT"},"isRemote":false},{"id":"43","jobOpeningName":"Rower","location":{"city":"Austin","state":"TX"},"isRemote":true}]}`

const bambooDetailFixture = `{"meta":{},"result":{"jobOpening":{"jobOpeningName":"Woodworker","datePosted":"2026-08-01","description":"<p>Sand <b>wood</b>.</p>","minimumExperience":"Entry-level","compensation":"","employmentStatusLabel":"Full Time","departmentLabel":"Shop"}}}`

func TestBambooDetectAndFetch(t *testing.T) {
	bh := BambooHR{Fetcher: &fakeFetcher{responses: map[string]string{
		"https://concept2.bamboohr.com/careers/list":      bambooListFixture,
		"https://concept2.bamboohr.com/careers/42/detail": bambooDetailFixture,
		// 43 has no canned detail → must degrade, not fail.
	}}}
	b := Board{Name: "concept2", Company: "Concept2", URL: "https://concept2.bamboohr.com/careers"}
	p, hit, err := DetectProvider(b)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if p.Name() != "bamboohr" {
		t.Fatalf("provider = %s", p.Name())
	}
	results, err := bh.Fetch(context.Background(), b, *hit, FetchOpts{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	// Job 42: detail enriched.
	enriched := results[0]
	if enriched.Date != "2026-08-01" {
		t.Fatalf("date = %q, want 2026-08-01 from detail", enriched.Date)
	}
	if enriched.Description != "Sand wood ." {
		t.Fatalf("description = %q, want HTML-stripped", enriched.Description)
	}
	if enriched.Metadata["experience"] != "Entry-level" || enriched.Metadata["employmentType"] != "Full Time" {
		t.Fatalf("metadata = %v", enriched.Metadata)
	}
	// Job 43: detail fetch failed → posting survives dateless.
	if results[1].Title != "Rower" || results[1].Date != "" {
		t.Fatalf("degraded posting wrong: %+v", results[1])
	}
	if results[0].URL != "https://concept2.bamboohr.com/careers/42" {
		t.Fatalf("url = %q", results[0].URL)
	}
}

const workdayFixture = `{"total":2,"jobPostings":[
  {"title":"Staff Engineer","bulletOne":"Posted 3 Days Ago","bulletTwo":"San Francisco, CA","externalPath":"/job/SF/Staff-Engineer_JR1"},
  {"title":"PM","bulletOne":"Posted Today","bulletTwo":"Remote, US","externalPath":"/job/US/PM_JR2"}
]}`

func TestWorkdayDetectAndFetch(t *testing.T) {
	w := Workday{Fetcher: &fakeFetcher{responses: map[string]string{
		"https://salesforce.wd12.myworkdayjobs.com/wday/cxs/salesforce/Slack/jobs": workdayFixture,
	}}}
	b := Board{Name: "slack", Company: "Slack", URL: "https://salesforce.wd12.myworkdayjobs.com/Slack/"}
	p, hit, err := DetectProvider(b)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if p.Name() != "workday" || !strings.Contains(hit.API, "/wday/cxs/salesforce/Slack/jobs") {
		t.Fatalf("provider=%s api=%s", p.Name(), hit.API)
	}
	results, err := w.Fetch(context.Background(), b, *hit, FetchOpts{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	if results[0].Date == "" || results[1].Date == "" {
		t.Fatalf("posted-ago dates not parsed: %+v", results)
	}
	if !strings.HasPrefix(results[0].URL, "https://salesforce.wd12.myworkdayjobs.com/en-US/Slack/job/") {
		t.Fatalf("url = %q", results[0].URL)
	}
}

func TestWorkdayLocaleAndInstanceHandling(t *testing.T) {
	// Locale-prefixed path still yields the right site.
	c, ok := workdayCoordFromBoard(Board{URL: "https://acme.wd3.myworkdayjobs.com/en-US/Careers"})
	if !ok || c.tenant != "acme" || c.instance != "wd3" || c.site != "Careers" {
		t.Fatalf("coord = %+v ok=%v", c, ok)
	}
	// Instance-less host parses with an empty instance (auto-probed later).
	c, ok = workdayCoordFromBoard(Board{URL: "https://salesforce.myworkdayjobs.com/Slack"})
	if !ok || c.tenant != "salesforce" || c.instance != "" || c.site != "Slack" {
		t.Fatalf("coord = %+v ok=%v", c, ok)
	}
	// Garbage hosts must not parse.
	for _, bad := range []string{
		"https://salesforce.wd3.myworkdayjobs.com/", // no site
		"https://salesforce.wd3.myworkdayjobs.com/wday/x",
		"https://a.b.wd3.myworkdayjobs.com/Site", // 3 dot-parts
	} {
		if _, ok := workdayCoordFromBoard(Board{URL: bad}); ok {
			t.Fatalf("%q parsed; want reject", bad)
		}
	}
}

func TestWorkdayInstanceAutoProbe(t *testing.T) {
	// wd1 rejects, wd5 answers: the probe must find wd5 AND keep its page
	// (the first-page-drop bug this test guards against).
	f := &fakeFetcher{responses: map[string]string{
		"https://acme.wd1.myworkdayjobs.com/wday/cxs/acme/Careers/jobs": `{"total":0,"jobPostings":[]}`,
		"https://acme.wd5.myworkdayjobs.com/wday/cxs/acme/Careers/jobs": workdayFixture,
	}}
	w := Workday{Fetcher: f}
	b := Board{Name: "acme", Company: "Acme", URL: "https://acme.myworkdayjobs.com/Careers"}
	p, hit, err := DetectProvider(b)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if p.Name() != "workday" || hit.API != "" {
		t.Fatalf("detect on unpinned instance: provider=%s api=%q", p.Name(), hit.API)
	}
	results, err := w.Fetch(context.Background(), b, *hit, FetchOpts{})
	if err != nil {
		t.Fatalf("fetch with auto-probe: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 (probe page must count)", len(results))
	}
	probed := false
	for _, req := range f.requested {
		if strings.HasPrefix(req, "https://acme.wd1.") {
			probed = true
		}
	}
	if !probed {
		t.Fatal("fetch never probed wd1")
	}
}

func TestDetectProviderUnknown(t *testing.T) {
	if _, _, err := DetectProvider(Board{Name: "x", URL: "https://example.com/careers"}); err != ErrNotMatched {
		t.Fatalf("err = %v, want ErrNotMatched", err)
	}
}

func TestWorkdayPolicyBlocksForeignHost(t *testing.T) {
	w := Workday{}
	for _, u := range []string{
		"http://salesforce.wd12.myworkdayjobs.com/x",   // not https
		"https://evil.example.com/wday/cxs/a/b/jobs",   // foreign host
		"https://evil.myworkdayjobs.com.attacker.io/x", // suffix trick
	} {
		if _, err := url.Parse(u); err != nil {
			t.Fatalf("parse %s: %v", u, err)
		}
		if err := w.policy()(u); err == nil {
			t.Fatalf("policy allowed %s", u)
		}
	}
	if err := w.policy()("https://salesforce.wd12.myworkdayjobs.com/wday/cxs/salesforce/Slack/jobs"); err != nil {
		t.Fatalf("policy rejected legitimate URL: %v", err)
	}
}
