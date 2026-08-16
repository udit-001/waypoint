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

// BambooHR scrapes a per-tenant BambooHR careers board. The public
// /careers/list endpoint carries only posting metadata (title, location);
// posting dates and descriptions live on the per-job /careers/{id}/detail
// endpoint. Fetch returns the lean list (no dates, no descriptions) and
// Detail is the on-demand enrichment step the agent runs on postings it's
// seriously considering.
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
	if b.URL == "" {
		return "", false
	}
	u, err := url.Parse(b.URL)
	if err != nil {
		return "", false
	}
	if u.Scheme != "https" || !bamboohrHostRE.MatchString(u.Hostname()) {
		return "", false
	}
	return "https://" + u.Hostname(), true
}

type bambooHRListResponse struct {
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

type bambooHRDetailResponse struct {
	Result struct {
		JobOpening struct {
			JobOpeningName     string `json:"jobOpeningName"`
			JobOpeningShareURL string `json:"jobOpeningShareUrl"`
			DepartmentLabel    string `json:"departmentLabel"`
			EmploymentLabel    string `json:"employmentStatusLabel"`
			Description        string `json:"description"`
			DatePosted         string `json:"datePosted"`
			MinimumExperience  string `json:"minimumExperience"`
			Compensation       string `json:"compensation"`
			Location           struct {
				City  string `json:"city"`
				State string `json:"state"`
			} `json:"location"`
		} `json:"jobOpening"`
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
	var resp bambooHRListResponse
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

// Detail fetches /careers/{id}/detail and returns the full posting: date
// posted, HTML-stripped description, and metadata (experience, employment
// type, department, compensation).
func (b BambooHR) Detail(ctx context.Context, bo Board, id string) (scraper.Result, error) {
	origin, ok := bambooOrigin(bo)
	if !ok {
		return scraper.Result{}, fmt.Errorf("bamboohr: not a bamboo board: %s", bo.URL)
	}
	f := b.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	raw, err := f.GetJSON(ctx, origin+"/careers/"+id+"/detail", b.policy())
	if err != nil {
		return scraper.Result{}, err
	}
	var resp bambooHRDetailResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return scraper.Result{}, err
	}
	d := resp.Result.JobOpening
	loc := strings.TrimSpace(d.Location.City + " " + d.Location.State)
	r := scraper.Result{
		ID:          id,
		Title:       strings.TrimSpace(d.JobOpeningName),
		Company:     bo.Company,
		Location:    loc,
		Date:        d.DatePosted,
		URL:         origin + "/careers/" + id,
		Description: strings.TrimSpace(scraper.HTMLToMarkdown(d.Description)),
	}
	meta := map[string]string{}
	if d.MinimumExperience != "" {
		meta["experience"] = d.MinimumExperience
	}
	if d.Compensation != "" {
		meta["compensation"] = d.Compensation
	}
	if d.EmploymentLabel != "" {
		meta["employmentType"] = d.EmploymentLabel
	}
	if d.DepartmentLabel != "" {
		meta["department"] = d.DepartmentLabel
	}
	if len(meta) > 0 {
		r.Metadata = meta
	}
	return r, nil
}
