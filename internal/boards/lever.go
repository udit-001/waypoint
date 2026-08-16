package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// leverHosts is the SSRF allowlist for the Lever provider.
var leverHosts = []string{"api.lever.co", "api.eu.lever.co"}

// leverBoardRE matches a jobs.lever.co careers URL and extracts the slug.
var leverBoardRE = regexp.MustCompile(`^jobs\.((?:eu\.)?lever\.co)$`)

// Lever scrapes the public Lever postings API.
type Lever struct {
	Fetcher JSONFetcher
}

func init() {
	Register(Lever{})
}

func (Lever) Name() string { return "lever" }

// Detect claims a Board whose careers URL (or API pin) is a Lever host.
func (l Lever) Detect(b Board) (*DetectHit, error) {
	raw := b.URL
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	// An explicit API pin must already be an allowed API host.
	if containsHost(leverHosts, host) {
		return &DetectHit{API: raw}, nil
	}
	// Otherwise the careers URL must be a jobs.lever.co/<slug> page.
	m := leverBoardRE.FindStringSubmatch(host)
	if m == nil {
		return nil, nil
	}
	slug := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")[0]
	if slug == "" {
		return nil, nil
	}
	api := fmt.Sprintf("https://api.%s/v0/postings/%s", m[1], slug)
	return &DetectHit{API: api}, nil
}

type leverPosting struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Hosted  string `json:"hostedUrl"`
	Created int64  `json:"createdAt"`
	Cat     struct {
		Location string `json:"location"`
	} `json:"categories"`
	Description string `json:"descriptionPlain"`
}

// policy returns the SSRF host policy for Lever.
func (Lever) policy() HostPolicy {
	allowed := map[string]bool{}
	for _, h := range leverHosts {
		allowed[h] = true
	}
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if u.Scheme != "https" {
			return fmt.Errorf("lever: URL must use https")
		}
		if !allowed[u.Hostname()] {
			return fmt.Errorf("lever: untrusted hostname %q", u.Hostname())
		}
		return nil
	}
}

func (l Lever) Fetch(ctx context.Context, b Board, hit DetectHit, opts FetchOpts) ([]scraper.Result, error) {
	f := l.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	raw, err := f.GetJSON(ctx, hit.API, l.policy())
	if err != nil {
		return nil, err
	}
	var postings []leverPosting
	if err := json.Unmarshal(raw, &postings); err != nil {
		return nil, err
	}
	results := make([]scraper.Result, 0, len(postings))
	for _, p := range postings {
		if strings.TrimSpace(p.Text) == "" || strings.TrimSpace(p.Hosted) == "" {
			continue
		}
		r := scraper.Result{
			ID:          p.ID,
			Title:       strings.TrimSpace(p.Text),
			Company:     b.Company,
			Location:    strings.TrimSpace(p.Cat.Location),
			URL:         strings.TrimSpace(p.Hosted),
			Description: strings.TrimSpace(p.Description),
		}
		if p.Created > 0 {
			r.Date = time.UnixMilli(p.Created).Format("2006-01-02")
		}
		if r.ID == "" {
			r.ID = r.URL
		}
		results = append(results, r)
	}
	results = scraper.FilterByRecency(results, opts.JobAgeDays, time.Time{})
	return scraper.Truncate(results, opts.Limit), nil
}

// Detail fetches /v0/postings/{slug}/{id}. The Lever postings API returns
// the full body in the list, so Detail is mostly a single-posting lookup
// (refresh from the live API); on detail-only fields (categories, plain
// description) the live fetch is authoritative when the staged copy is stale.
func (l Lever) Detail(ctx context.Context, b Board, id string) (scraper.Result, error) {
	raw := b.URL
	if raw == "" {
		return scraper.Result{}, fmt.Errorf("lever: board URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return scraper.Result{}, err
	}
	m := leverBoardRE.FindStringSubmatch(u.Hostname())
	if m == nil {
		return scraper.Result{}, fmt.Errorf("lever: cannot derive slug from %s", raw)
	}
	slug := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")[0]
	if slug == "" {
		return scraper.Result{}, fmt.Errorf("lever: empty slug in %s", raw)
	}
	f := l.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	api := fmt.Sprintf("https://api.%s/v0/postings/%s/%s", m[1], slug, id)
	respBytes, err := f.GetJSON(ctx, api, l.policy())
	if err != nil {
		return scraper.Result{}, err
	}
	var p leverPosting
	if err := json.Unmarshal(respBytes, &p); err != nil {
		return scraper.Result{}, err
	}
	r := scraper.Result{
		ID:          p.ID,
		Title:       strings.TrimSpace(p.Text),
		Company:     b.Company,
		Location:    strings.TrimSpace(p.Cat.Location),
		URL:         strings.TrimSpace(p.Hosted),
		Description: strings.TrimSpace(p.Description),
	}
	if p.Created > 0 {
		r.Date = time.UnixMilli(p.Created).Format("2006-01-02")
	}
	if r.ID == "" {
		r.ID = r.URL
	}
	return r, nil
}
