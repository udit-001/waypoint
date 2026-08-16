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

// BambooHR scrapes the public per-tenant BambooHR careers list API.
// Per-tenant subdomains are the variable part, so SSRF defense uses a regex
// match on <safe-tenant>.bamboohr.com rather than a static allowlist.
type BambooHR struct {
	Fetcher JSONFetcher
}

func init() {
	Register(BambooHR{})
}

func (BambooHR) Name() string { return "bamboohr" }

var bamboohrHostRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.bamboohr\.com$`)

// Detect claims a Board whose URL is a <tenant>.bamboohr.com host.
func (b BambooHR) Detect(bo Board) (*DetectHit, error) {
	origin, ok := bambooOrigin(bo)
	if !ok {
		return nil, nil
	}
	return &DetectHit{API: origin + "/careers/list"}, nil
}

// bambooOrigin returns the tenant origin (https://<tenant>.bamboohr.com)
// when the board matches, else "".
func bambooOrigin(b Board) (string, bool) {
	raw := b.URL
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Scheme != "https" || !bamboohrHostRE.MatchString(u.Hostname()) {
		return "", false
	}
	return "https://" + u.Hostname(), true
}

type bambooHRResponse struct {
	Result []struct {
		ID             string `json:"id"`
		JobOpeningName string `json:"jobOpeningName"`
		Location       struct {
			City  string `json:"city"`
			State string `json:"state"`
		} `json:"location"`
		IsRemote *bool `json:"isRemote"`
	} `json:"result"`
}

// policy returns the SSRF host policy for BambooHR.
func (BambooHR) policy() HostPolicy {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if u.Scheme != "https" {
			return fmt.Errorf("bamboohr: URL must use https")
		}
		if !bamboohrHostRE.MatchString(u.Hostname()) {
			return fmt.Errorf("bamboohr: untrusted hostname %q", u.Hostname())
		}
		return nil
	}
}

func (b BambooHR) Fetch(ctx context.Context, bo Board, hit DetectHit, opts FetchOpts) ([]scraper.Result, error) {
	f := b.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	raw, err := f.GetJSON(ctx, hit.API, b.policy())
	if err != nil {
		return nil, err
	}
	var resp bambooHRResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	origin, _ := bambooOrigin(bo)
	results := make([]scraper.Result, 0, len(resp.Result))
	for _, j := range resp.Result {
		if strings.TrimSpace(j.JobOpeningName) == "" || strings.TrimSpace(j.ID) == "" {
			continue
		}
		loc := strings.TrimSpace(j.Location.City + " " + j.Location.State)
		if j.IsRemote != nil && *j.IsRemote {
			if loc != "" {
				loc += ", "
			}
			loc += "Remote"
		}
		results = append(results, scraper.Result{
			ID:       j.ID,
			Title:    strings.TrimSpace(j.JobOpeningName),
			Company:  bo.Company,
			Location: loc,
			URL:      origin + "/careers/" + strings.TrimSpace(j.ID),
		})
	}
	results = scraper.FilterByRecency(results, opts.JobAgeDays, time.Time{})
	return scraper.Truncate(results, opts.Limit), nil
}
