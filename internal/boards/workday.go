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

// cxsJobURL builds the CXS detail endpoint for a posting slug. The slug is
// the tail of externalPath, e.g. "Sr-Staff-Software-Engineer--Android_JR355162".
func (c workdayCoord) cxsJobURL(instance, slug string) string {
	return fmt.Sprintf("https://%s.%s.myworkdayjobs.com/wday/cxs/%s/%s/job/%s",
		c.tenant, instance, c.tenant, c.site, slug)
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

// workdayJob is one posting row in the CXS list response. The list carries
// a relative-date label and a slug; the full description and an absolute
// startDate live on the detail endpoint.
type workdayJob struct {
	Title         string   `json:"title"`
	PostedOn      string   `json:"postedOn"`      // "Posted N Days Ago" / "Posted Today"
	LocationsText string   `json:"locationsText"` // single string, may be "N Locations"
	BulletFields  []string `json:"bulletFields"`  // bulletFields[0] is the job req id
	ExternalPath  string   `json:"externalPath"`  // "/job/.../<postingSlug>_JR<n>"
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

// slugFromPath extracts the detail slug (the JSON Detail endpoint key) from
// an externalPath like "/job/SF/Sr-Staff-Software-Engineer--Android_JR355162".
// The slug is the last path segment.
func slugFromPath(externalPath string) string {
	segs := strings.Split(strings.Trim(externalPath, "/"), "/")
	if len(segs) == 0 {
		return ""
	}
	return segs[len(segs)-1]
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
			slug := slugFromPath(j.ExternalPath)
			if slug == "" {
				continue
			}
			boardURL := fmt.Sprintf("https://%s.%s.myworkdayjobs.com/%s%s",
				c.tenant, instance, c.site, j.ExternalPath)
			meta := map[string]string{}
			if len(j.BulletFields) > 0 && j.BulletFields[0] != "" {
				meta["reqId"] = j.BulletFields[0]
			}
			results = append(results, scraper.Result{
				ID:       slug, // detail endpoint key
				Title:    strings.TrimSpace(j.Title),
				Company:  b.Company,
				Location: strings.TrimSpace(j.LocationsText),
				Date:     parsePostedAgo(j.PostedOn, now),
				URL:      boardURL,
				Metadata: meta,
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

// workdayDetailResponse is the GET /wday/cxs/{tenant}/{site}/job/{slug}
// payload. jobPostingInfo carries the full description, an absolute
// startDate, and structured fields.
type workdayDetailResponse struct {
	JobPostingInfo struct {
		ID             string `json:"id"`
		Title          string `json:"title"`
		JobDescription string `json:"jobDescription"`
		Location       string `json:"location"`
		PostedOn       string `json:"postedOn"`
		StartDate      string `json:"startDate"` // actual ISO date (2026-08-13)
		TimeType       string `json:"timeType"`
		JobReqID       string `json:"jobReqId"`
		JobPostingID   string `json:"jobPostingId"`
		RemoteType     string `json:"remoteType"`
		ExternalURL    string `json:"externalUrl"`
		Country        struct {
			Descriptor string `json:"descriptor"`
		} `json:"country"`
	} `json:"jobPostingInfo"`
}

// Detail fetches /wday/cxs/{tenant}/{site}/job/{slug} and returns the full
// posting body. The slug is the scraper.Result.ID returned by Fetch. When
// the board URL pins no instance, the known-instance probe runs once here
// to satisfy the detail URL.
func (w Workday) Detail(ctx context.Context, b Board, id string) (scraper.Result, error) {
	f := w.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	c, ok := workdayCoordFromBoard(b)
	if !ok {
		return scraper.Result{}, fmt.Errorf("workday: board URL lost its coordinates")
	}
	instance := c.instance
	if instance == "" {
		for _, inst := range workdayInstances {
			if _, err := w.page(ctx, f, c, inst, 0); err == nil {
				instance = inst
				break
			}
		}
		if instance == "" {
			return scraper.Result{}, fmt.Errorf("workday: no live instance found for tenant %q", c.tenant)
		}
	}
	origin := fmt.Sprintf("https://%s.%s.myworkdayjobs.com", c.tenant, instance)
	raw, err := f.GetJSON(ctx, c.cxsJobURL(instance, id), w.policy())
	if err != nil {
		return scraper.Result{}, err
	}
	var resp workdayDetailResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return scraper.Result{}, err
	}
	d := resp.JobPostingInfo
	r := scraper.Result{
		ID:          id,
		Title:       strings.TrimSpace(d.Title),
		Company:     b.Company,
		Location:    strings.TrimSpace(d.Location),
		Description: strings.TrimSpace(scraper.HTMLToMarkdown(d.JobDescription)),
	}
	if d.ExternalURL != "" {
		r.URL = d.ExternalURL
	} else {
		r.URL = origin + "/" + c.site + "/details/" + id
	}
	// Prefer the absolute startDate ("2026-08-13"); fall back to parsing
	// the relative "Posted N Days Ago" label.
	if d.StartDate != "" {
		r.Date = d.StartDate
	} else if d.PostedOn != "" {
		r.Date = parsePostedAgo(d.PostedOn, time.Now().UTC())
	}
	meta := map[string]string{}
	if d.JobReqID != "" {
		meta["reqId"] = d.JobReqID
	}
	if d.TimeType != "" {
		meta["employmentType"] = d.TimeType
	}
	if d.RemoteType != "" {
		meta["remoteType"] = d.RemoteType
	}
	if d.Country.Descriptor != "" {
		meta["country"] = d.Country.Descriptor
	}
	if len(meta) > 0 {
		r.Metadata = meta
	}
	return r, nil
}
