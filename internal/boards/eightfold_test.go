package boards

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// eightfoldCareersFixture is the citi careers page shell with the embedded
// pcsx-data config block. The domain lives inside the JSON as HTML-escaped
// quotes — the exact shape the SPA serves.
const eightfoldCareersFixture = `<!doctype html><html><head><title>Citi Careers</title></head>
<body>
<div id="root"></div>
<code id="pcsx-data" style="display:none;" data-nosnippet>{&#34;domain&#34;: &#34;citi.com&#34;, &#34;navbar-text-color&#34;: &#34;#ffffff&#34;}</code>
<script src="/gen/js/app.js"></script>
</body></html>`

// eightfoldPageFixture builds a pcsx search response with n positions.
// Position i gets id 1000+n*offsetIdx+i and posts at epoch+offsetIdx seconds,
// where offsetIdx is the page offset in pages (0-based). A short page
// (n < eightfoldPageSize) is what stops pagination in the provider.
func eightfoldPageFixture(n, offsetIdx int, epoch int64) string {
	var sb strings.Builder
	sb.WriteString(`{"status":200,"data":{"count":25,"positions":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		id := int64(1000 + offsetIdx*10 + i)
		loc := "Mumbai, MH, IN"
		if i%2 == 1 {
			loc = "Pune, MH, IN"
		}
		sb.WriteString(`{"id":` + i64s(id) + `,"displayJobId":"` + i64s(id*7) +
			`","name":"Software Engineer ` + i64s(id) +
			`","standardizedLocations":["` + loc +
			`"],"postedTs":` + i64s(epoch+int64(offsetIdx*10+i)) +
			`,"department":"Engineering","atsJobId":"` + i64s(id*13) +
			`","workLocationOption":"hybrid"}`)
	}
	sb.WriteString(`]}}`)
	return sb.String()
}

func i64s(i int64) string {
	return strconv.FormatInt(i, 10)
}

const eightfoldDetailFixture = `{"status":200,"data":{"id":1000,"displayJobId":"7000","name":"Software Engineer 1000","jobDescription":"<p>Build the <b>modeling</b> platform.</p>","publicUrl":"https://citi.eightfold.ai/careers/job/1000","standardizedLocations":["Mumbai, MH, IN"],"department":"Engineering","workLocationOption":"hybrid"}}`

func TestEightfoldDetect(t *testing.T) {
	p, hit, err := DetectProvider(Board{URL: "https://citi.eightfold.ai/careers"})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if p.Name() != "eightfold" || hit.API != "" {
		t.Fatalf("provider=%s api=%q, want eightfold with empty API (needs probe)", p.Name(), hit.API)
	}

	// Foreign hosts are not claimed.
	if _, _, err := DetectProvider(Board{URL: "https://jobs.citi.com/"}); err == nil {
		t.Fatal("DetectProvider claimed a non-eightfold host")
	}
}

func TestEightfoldDomainFromCareers(t *testing.T) {
	dom, err := domainFromCareers([]byte(eightfoldCareersFixture))
	if err != nil {
		t.Fatalf("domainFromCareers: %v", err)
	}
	if dom != "citi.com" {
		t.Fatalf("domain = %q, want citi.com", dom)
	}
}

// nowEpoch anchors fixture timestamps to the machine clock so recency
// assertions stay valid at any run date.
func nowEpoch() int64 { return time.Now().UTC().Unix() }

func eightfoldTestFetch(t *testing.T, opts FetchOpts) []scraper.Result {
	t.Helper()
	now := nowEpoch()
	f := &fakeFetcher{responses: map[string]string{
		"https://citi.eightfold.ai/careers": eightfoldCareersFixture,
		// Page 0: full 10 rows (pagination does not stop here).
		"https://citi.eightfold.ai/api/pcsx/search?domain=citi.com&query=&location=&start=0&num=10&sort_by=newest": eightfoldPageFixture(10, 0, now),
		// Page 1: 5 rows, posted 5 days before page 0 (short page → pagination
		// stops; older rows → recency filter can drop them).
		"https://citi.eightfold.ai/api/pcsx/search?domain=citi.com&query=&location=&start=10&num=10&sort_by=newest": eightfoldPageFixture(5, 1, now-5*86400),
	}}
	e := Eightfold{Fetcher: f}
	b := Board{Name: "citi", Company: "Citi", URL: "https://citi.eightfold.ai/careers"}
	results, err := e.Fetch(context.Background(), b, DetectHit{}, opts)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	return results
}

func TestEightfoldFetchPaginatesAndNormalizes(t *testing.T) {
	// Full sweep: both pages fetched and returned (15 rows), newest first.
	results := eightfoldTestFetch(t, FetchOpts{})
	if len(results) != 15 {
		t.Fatalf("got %d results, want 15", len(results))
	}
	first := results[0]
	if first.ID != "1000" || first.Title != "Software Engineer 1000" {
		t.Fatalf("first result = %+v", first)
	}
	if first.Company != "Citi" || first.Location != "Mumbai, MH, IN" {
		t.Fatalf("normalization off: %+v", first)
	}
	if first.URL != "https://citi.eightfold.ai/careers/job/1000" {
		t.Fatalf("bad url: %s", first.URL)
	}
	if first.Date == "" {
		t.Fatal("date not derived from postedTs")
	}
	if first.Metadata["department"] != "Engineering" || first.Metadata["work_location"] != "hybrid" {
		t.Fatalf("metadata off: %+v", first.Metadata)
	}

	// Limit trims after pagination + recency.
	if got := len(eightfoldTestFetch(t, FetchOpts{Limit: 3})); got != 3 {
		t.Fatalf("limit: got %d, want 3", got)
	}
}

func TestEightfoldRecencyFilter(t *testing.T) {
	// All 15 rows post within epoch..epoch+14 (same day), so a 30-day
	// window keeps every row — proving the filter is wired to postedTs
	// and doesn't drop a full sweep.
	results := eightfoldTestFetch(t, FetchOpts{JobAgeDays: 30})
	if len(results) != 15 {
		t.Fatalf("30-day window: got %d, want 15", len(results))
	}
	// A 1-day window drops page 1 (posted 5 days ago) — proving the filter
	// is wired to postedTs.
	results = eightfoldTestFetch(t, FetchOpts{JobAgeDays: 1})
	if len(results) != 10 {
		t.Fatalf("1-day window: got %d, want 10 (page 1 is 5 days old)", len(results))
	}
}

// TestEightfoldEarlyStop verifies the newest-first pagination early-stop:
// a full page whose newest row already predates the recency window ends
// pagination immediately, before the posting-count stop would. The fake
// responds only to pages 0–1 while count claims 25, so without the
// early-stop the loop would request page 2 and error; with it, the stale
// page 1 breaks the loop first.
func TestEightfoldEarlyStop(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{
		"https://citi.eightfold.ai/careers": eightfoldCareersFixture,
		"https://citi.eightfold.ai/api/pcsx/search?domain=citi.com&query=&location=&start=0&num=10&sort_by=newest":  eightfoldPageFixture(10, 0, nowEpoch()),          // page 0: full, fresh
		"https://citi.eightfold.ai/api/pcsx/search?domain=citi.com&query=&location=&start=10&num=10&sort_by=newest": eightfoldPageFixture(10, 1, nowEpoch()-11*86400), // page 1: full, 11 days old
	}}
	e := Eightfold{Fetcher: f}
	b := Board{Name: "citi", Company: "Citi", URL: "https://citi.eightfold.ai/careers"}

	// Control — no window: pages through both fixtures, then requests page 2
	// (start=20, no fixture) and fails on the missing response. This proves
	// count=25 outlives the provided pages, so the windowed run genuinely
	// exercises the early-stop.
	if _, err := e.Fetch(context.Background(), b, DetectHit{}, FetchOpts{}); err == nil {
		t.Fatal("control: expected error requesting missing page 2")
	}

	// Windowed — 7 days: page 0 (1 day old) is fresh, page 1 (11 days old)
	// is stale → an early-stop fires on page 1, page 2 is never requested,
	// and the final trim keeps only page 0's rows.
	results, err := e.Fetch(context.Background(), b, DetectHit{}, FetchOpts{JobAgeDays: 7})
	if err != nil {
		t.Fatalf("fetch (window): %v", err)
	}
	if len(results) != 10 {
		t.Fatalf("windowed: got %d, want 10 (stale page 1 trimmed; page 2 never fetched)", len(results))
	}
}

func TestEightfoldDetail(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{
		"https://citi.eightfold.ai/api/pcsx/position_details": eightfoldDetailFixture,
	}}
	e := Eightfold{Fetcher: f}
	b := Board{Name: "citi", Company: "Citi", URL: "https://citi.eightfold.ai/careers"}

	r, err := e.Detail(context.Background(), b, "1000")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if r.Title != "Software Engineer 1000" {
		t.Fatalf("title = %q", r.Title)
	}
	if !strings.Contains(r.Description, "**modeling**") || strings.Contains(r.Description, "<b>") {
		t.Fatalf("description not html→markdown: %q", r.Description)
	}
	if r.URL != "https://citi.eightfold.ai/careers/job/1000" {
		t.Fatalf("url = %q", r.URL)
	}
	if r.Company != "Citi" {
		t.Fatalf("company = %q", r.Company)
	}
}

func TestEightfoldPolicyRejectsForeignHost(t *testing.T) {
	e := Eightfold{Fetcher: &fakeFetcher{}}
	if err := e.policy()("https://evil.example.com/api/pcsx/search"); err == nil {
		t.Fatal("policy allowed foreign host")
	}
	if err := e.policy()("http://citi.eightfold.ai/api/pcsx/search"); err == nil {
		t.Fatal("policy allowed http")
	}
	if err := e.policy()("https://citi.eightfold.ai/api/pcsx/search"); err != nil {
		t.Fatalf("policy rejected legit host: %v", err)
	}
}
