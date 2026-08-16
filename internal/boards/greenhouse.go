package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// greenhouseHosts is the SSRF allowlist for the Greenhouse provider.
var greenhouseHosts = []string{
	"boards-api.greenhouse.io",
	"boards.greenhouse.io",
	"job-boards.greenhouse.io",
	"job-boards.eu.greenhouse.io",
}

// Greenhouse scrapes the public Greenhouse boards API. The list endpoint
// returns metadata only (title, location, updated_at); the per-job detail
// endpoint adds the full HTML description and the department list.
type Greenhouse struct {
	Fetcher JSONFetcher
}

func init() {
	Register(Greenhouse{})
}

func (Greenhouse) Name() string { return "greenhouse" }

// Detect claims a Board whose careers URL (or API pin) is a Greenhouse host.
func (g Greenhouse) Detect(b Board) (*DetectHit, error) {
	raw := b.URL
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !containsHost(greenhouseHosts, u.Hostname()) {
		return nil, nil
	}
	// Host already checked; the board token is the first path segment.
	board := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	if board == "" || board == "embed" || strings.Contains(board, ".") {
		return nil, nil
	}
	api := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", board)
	return &DetectHit{API: api}, nil
}

// greenhouseListResponse is the /jobs list response: metadata only.
type greenhouseListResponse struct {
	Jobs []struct {
		ID          json.Number `json:"id"`
		Title       string      `json:"title"`
		AbsoluteURL string      `json:"absolute_url"`
		Location    struct {
			Name string `json:"name"`
		} `json:"location"`
		UpdatedAt string `json:"updated_at"`
	} `json:"jobs"`
}

// greenhouseDetailResponse is the /jobs/{id} detail response. It carries
// the full HTML content and the structured department list the list
// endpoint omits.
type greenhouseDetailResponse struct {
	ID          json.Number `json:"id"`
	Title       string      `json:"title"`
	AbsoluteURL string      `json:"absolute_url"`
	Content     string      `json:"content"` // full HTML description
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
	FirstPublished string `json:"first_published"`
	UpdatedAt      string `json:"updated_at"`
	Departments    []struct {
		Name string `json:"name"`
	} `json:"departments"`
}

// policy returns the SSRF host policy for Greenhouse.
func (Greenhouse) policy() HostPolicy {
	allowed := map[string]bool{}
	for _, h := range greenhouseHosts {
		allowed[h] = true
	}
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if u.Scheme != "https" {
			return fmt.Errorf("greenhouse: URL must use https")
		}
		if !allowed[u.Hostname()] {
			return fmt.Errorf("greenhouse: untrusted hostname %q", u.Hostname())
		}
		return nil
	}
}

func (g Greenhouse) Fetch(ctx context.Context, b Board, hit DetectHit, opts FetchOpts) ([]scraper.Result, error) {
	f := g.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	raw, err := f.GetJSON(ctx, hit.API, g.policy())
	if err != nil {
		return nil, err
	}
	var resp greenhouseListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	results := make([]scraper.Result, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if strings.TrimSpace(j.Title) == "" || strings.TrimSpace(j.AbsoluteURL) == "" {
			continue
		}
		r := scraper.Result{
			ID:       j.ID.String(),
			Title:    strings.TrimSpace(j.Title),
			Company:  b.Company,
			Location: strings.TrimSpace(j.Location.Name),
			Date:     parseAPIDate(j.UpdatedAt),
			URL:      strings.TrimSpace(j.AbsoluteURL),
		}
		if r.ID == "" {
			r.ID = r.URL
		}
		results = append(results, r)
	}
	results = scraper.FilterByRecency(results, opts.JobAgeDays, time.Time{})
	return scraper.Truncate(results, opts.Limit), nil
}

// Detail fetches /v1/boards/{board}/jobs/{id} and returns the full posting
// body: HTML-stripped description, the preferred published date, and a
// department-list metadata entry.
func (g Greenhouse) Detail(ctx context.Context, b Board, id string) (scraper.Result, error) {
	raw := b.URL
	if raw == "" {
		return scraper.Result{}, fmt.Errorf("greenhouse: board URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return scraper.Result{}, err
	}
	board := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	if board == "" || board == "embed" {
		return scraper.Result{}, fmt.Errorf("greenhouse: cannot derive board token from %s", raw)
	}
	f := g.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	api := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs/%s", board, id)
	respBytes, err := f.GetJSON(ctx, api, g.policy())
	if err != nil {
		return scraper.Result{}, err
	}
	var d greenhouseDetailResponse
	if err := json.Unmarshal(respBytes, &d); err != nil {
		return scraper.Result{}, err
	}
	r := scraper.Result{
		ID:          id,
		Title:       strings.TrimSpace(d.Title),
		Company:     b.Company,
		Location:    strings.TrimSpace(d.Location.Name),
		URL:         strings.TrimSpace(d.AbsoluteURL),
		Description: strings.TrimSpace(scraper.HTMLToMarkdown(d.Content)),
	}
	// Prefer FirstPublished (when the posting went live); fall back to UpdatedAt.
	if d.FirstPublished != "" {
		r.Date = parseAPIDate(d.FirstPublished)
	} else if d.UpdatedAt != "" {
		r.Date = parseAPIDate(d.UpdatedAt)
	}
	var deptNames []string
	for _, dep := range d.Departments {
		if n := strings.TrimSpace(dep.Name); n != "" {
			deptNames = append(deptNames, n)
		}
	}
	if len(deptNames) > 0 {
		r.Metadata = map[string]string{"department": strings.Join(deptNames, ", ")}
	}
	return r, nil
}

// parseAPIDate normalizes an RFC3339 timestamp to YYYY-MM-DD. Empty or
// unparseable values return "". Boards that don't expose a date surface
// yield a blank Date, which the recency filter treats conservatively.
func parseAPIDate(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// containsHost reports whether s lies within hosts.
func containsHost(hosts []string, s string) bool {
	for _, h := range hosts {
		if h == s {
			return true
		}
	}
	return false
}
