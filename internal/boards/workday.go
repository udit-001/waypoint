package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// workdayInstances is the ordered set of Workday tenant instances to probe
// when the board URL does not pin one. Mirrors career-ops' wd1..wd103 list.
var workdayInstances = []string{"wd1", "wd2", "wd3", "wd5", "wd10", "wd12", "wd101", "wd103"}

// localeRE matches a locale path prefix like en-US.
var localeRE = regexp.MustCompile(`^[a-z]{2}-[A-Z]{2}$`)

// workdayCoord captures the three Workday board coordinates.
type workdayCoord struct {
	tenant   string
	site     string
	instance string // may be empty → auto-probe at fetch time
}

// workdayCoordFromBoard parses tenant (and instance, when pinned) plus the
// site out of a myworkdayjobs.com URL. Both forms are accepted:
//
//	https://tenant.wd3.myworkdayjobs.com/Site
//	https://tenant.myworkdayjobs.com/Site   (instance auto-probed later)
func workdayCoordFromBoard(b Board) (workdayCoord, bool) {
	raw := b.URL
	if raw == "" {
		return workdayCoord{}, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return workdayCoord{}, false
	}
	if u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".myworkdayjobs.com") {
		return workdayCoord{}, false
	}
	host := strings.TrimSuffix(u.Hostname(), ".myworkdayjobs.com")
	parts := strings.Split(host, ".")
	var c workdayCoord
	switch len(parts) {
	case 1:
		c.tenant = parts[0]
	case 2:
		c.tenant, c.instance = parts[0], parts[1]
	default:
		return workdayCoord{}, false
	}
	if c.tenant == "" || (c.instance != "" && !strings.HasPrefix(c.instance, "wd")) {
		return workdayCoord{}, false
	}
	// Site: first path segment, skipping a locale prefix like en-US.
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) == 0 {
		return workdayCoord{}, false
	}
	if localeRE.MatchString(segs[0]) {
		segs = segs[1:]
	}
	if len(segs) == 0 || segs[0] == "" || segs[0] == "wday" {
		return workdayCoord{}, false
	}
	c.site = segs[0]
	return c, true
}

// Workday scrapes the Workday DJobs CXS endpoint. Workday boards require a
// tenant + instance + site coordinate. The instance auto-probe is the heal
// path for tenant-host drift (companies migrate wd3→wd5 etc.).
type Workday struct {
	Fetcher JSONFetcher
}

func init() {
	Register(Workday{})
}

func (Workday) Name() string { return "workday" }

// Detect claims a Board whose URL is a myworkdayjobs.com host. The API pin
// is empty when the instance is unpinned — Fetch probes it then.
func (w Workday) Detect(b Board) (*DetectHit, error) {
	c, ok := workdayCoordFromBoard(b)
	if !ok {
		return nil, nil
	}
	return &DetectHit{API: c.cxsURL(c.instance)}, nil
}

// cxsURL builds the CXS jobs endpoint for a given instance. An empty
// instance yields "" — the caller must probe before use.
func (c workdayCoord) cxsURL(instance string) string {
	if instance == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.%s.myworkdayjobs.com/wday/cxs/%s/%s/jobs",
		c.tenant, instance, c.tenant, c.site)
}

// policy returns the SSRF host policy for Workday.
func (Workday) policy() HostPolicy {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if u.Scheme != "https" {
			return fmt.Errorf("workday: URL must use https")
		}
		if !strings.HasSuffix(u.Hostname(), ".myworkdayjobs.com") {
			return fmt.Errorf("workday: untrusted hostname %q", u.Hostname())
		}
		return nil
	}
}

// workdayBody is the CXS POST request payload.
type workdayBody struct {
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
	SearchText    string         `json:"searchText"`
	AppliedFacets map[string]any `json:"appliedFacets"`
}

// workdayResponse is the CXS POST response.
type workdayResponse struct {
	Total       int          `json:"total"`
	JobPostings []workdayJob `json:"jobPostings"`
}

type workdayJob struct {
	Title        string `json:"title"`
	BulletOne    string `json:"bulletOne"` // "Posted N Days Ago" lives here
	BulletTwo    string `json:"bulletTwo"` // often the location
	ExternalPath string `json:"externalPath"`
}

// postedAgoRE parses "Posted N Days Ago" / "Posted Today".
var postedAgoRE = regexp.MustCompile(`(?i)posted\s+today|posted\s+(\d+)\s+day`)

// parsePostedAgo converts a Workday relative-date bullet to YYYY-MM-DD.
func parsePostedAgo(label string, now time.Time) string {
	m := postedAgoRE.FindStringSubmatch(label)
	if m == nil {
		return ""
	}
	if strings.Contains(strings.ToLower(label), "today") {
		return now.Format("2006-01-02")
	}
	if n, err := strconv.Atoi(m[1]); err == nil {
		return now.AddDate(0, 0, -n).Format("2006-01-02")
	}
	return ""
}

const (
	workdayPageSize     = 20
	workdayMaxPages     = 100 // default; hard cap workdayHardMaxPages
	workdayHardMaxPages = 1500
)

func (w Workday) Fetch(ctx context.Context, b Board, hit DetectHit, opts FetchOpts) ([]scraper.Result, error) {
	f := w.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	c, ok := workdayCoordFromBoard(b)
	if !ok {
		return nil, fmt.Errorf("workday: board URL lost its coordinates")
	}

	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = b.MaxPages
	}
	if maxPages <= 0 {
		maxPages = workdayMaxPages
	}
	if maxPages > workdayHardMaxPages {
		maxPages = workdayHardMaxPages
	}

	// Resolve the instance when the board URL pinned none: probe the first
	// page on each known instance until one answers, and keep that page —
	// it is page 0 of the results.
	instance := c.instance
	var firstPage []workdayJob
	if instance == "" {
		for _, inst := range workdayInstances {
			jobs, err := w.page(ctx, f, c, inst, 0)
			if err == nil && len(jobs) > 0 {
				instance, firstPage = inst, jobs
				break
			}
		}
		if instance == "" {
			return nil, fmt.Errorf("workday: no live instance found for tenant %q", c.tenant)
		}
	}

	var results []scraper.Result
	now := time.Now().UTC()
	offset := 0
	for page := 0; page < maxPages; page++ {
		var jobs []workdayJob
		if firstPage != nil {
			jobs, firstPage = firstPage, nil
		} else {
			var err error
			jobs, err = w.page(ctx, f, c, instance, offset)
			if err != nil {
				return nil, err
			}
		}
		if len(jobs) == 0 {
			break
		}
		for _, j := range jobs {
			if strings.TrimSpace(j.Title) == "" || strings.TrimSpace(j.ExternalPath) == "" {
				continue
			}
			boardURL := fmt.Sprintf("https://%s.%s.myworkdayjobs.com/en-US/%s%s",
				c.tenant, instance, c.site, j.ExternalPath)
			results = append(results, scraper.Result{
				ID:       c.tenant + ":" + j.ExternalPath,
				Title:    strings.TrimSpace(j.Title),
				Company:  b.Company,
				Location: strings.TrimSpace(j.BulletTwo),
				Date:     parsePostedAgo(j.BulletOne, now),
				URL:      boardURL,
			})
		}
		offset += len(jobs)
		if len(jobs) < workdayPageSize {
			break
		}
	}
	results = scraper.FilterByRecency(results, opts.JobAgeDays, time.Time{})
	return scraper.Truncate(results, opts.Limit), nil
}

// page fetches one CXS page for a given instance and offset.
func (w Workday) page(ctx context.Context, f JSONFetcher, c workdayCoord, instance string, offset int) ([]workdayJob, error) {
	payload, err := json.Marshal(workdayBody{
		Limit:         workdayPageSize,
		Offset:        offset,
		SearchText:    "",
		AppliedFacets: map[string]any{},
	})
	if err != nil {
		return nil, err
	}
	origin := fmt.Sprintf("https://%s.%s.myworkdayjobs.com", c.tenant, instance)
	headers := map[string]string{
		"Accept-Language": "en-US,en;q=0.9",
		"Origin":          origin,
		"Referer":         origin + "/" + c.site + "/",
	}
	raw, err := f.PostJSON(ctx, c.cxsURL(instance), payload, headers, w.policy())
	if err != nil {
		return nil, err
	}
	var resp workdayResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.JobPostings, nil
}
