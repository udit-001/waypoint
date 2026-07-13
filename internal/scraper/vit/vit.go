package vit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	apiURL      = "https://public.zwayam.com/manageESQueries/searchJob"
	companyID   = "15196"
	companyURL  = "careers.vit.ac.in/"
	careersBase = "https://careers.vit.ac.in/vit/job/"
	sourceName  = "VIT Vellore"
)

type VIT struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(VIT{})
}

func (VIT) Name() string         { return "vit" }
func (VIT) Source() string       { return sourceName }
func (VIT) Categories() []string { return []string{"academic", "biotech"} }

func (n VIT) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	// Fetch from all categories
	categories := []string{"Faculty Recruitment", "Staff Recruitment", "Project Recruitment"}
	var allResults []scraper.Result

	for _, cat := range categories {
		results, err := n.fetchJobs(ctx, cat)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

type zwayamJob struct {
	JobTitle        string   `json:"jobTitle"`
	JobCode         int      `json:"jobCode"`
	JobURL          string   `json:"jobUrl"`
	Location        string   `json:"location"`
	ExperienceUI    string   `json:"experienceUIField"`
	SkillSet        string   `json:"skillSet"`
	MandatorySkills []string `json:"mandatorySkills"`
	Positions       int      `json:"positionsRequired"`
	CreatedDate     int64    `json:"createdDate"`
}

func (n VIT) fetchJobs(ctx context.Context, category string) ([]scraper.Result, error) {
	if n.Fetcher != nil {
		body, err := n.Fetcher.Fetch(ctx, apiURL)
		if err != nil {
			return nil, err
		}
		return parseZwayamJobs(body, category)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("id", companyID)
	writer.WriteField("companyUrl", companyURL)
	writer.WriteField("job", "empty")
	writer.WriteField("city", "undefined")
	writer.WriteField("userGeoLocation", "undefined")
	writer.WriteField("departmentName", "empty")
	writer.WriteField("fieldName", "allDeptHierarchy.dept3")
	writer.WriteField("fieldValue", category)
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Referer", "https://careers.vit.ac.in/")

	raw, err := scraper.DoWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	return parseZwayamJobs(raw, category)
}

func parseZwayamJobs(raw string, category string) ([]scraper.Result, error) {
	var jobs []struct {
		Source zwayamJob `json:"_source"`
	}
	if err := json.Unmarshal([]byte(raw), &jobs); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var results []scraper.Result
	for _, j := range jobs {
		s := j.Source
		if s.JobTitle == "" {
			continue
		}

		createdDate := ""
		if s.CreatedDate > 0 {
			createdDate = time.UnixMilli(s.CreatedDate).Format("2006-01-02")
		}

		skills := s.SkillSet
		if len(s.MandatorySkills) > 0 {
			skills = strings.Join(s.MandatorySkills, ", ")
		}

		url := careersBase + s.JobURL
		if !strings.HasPrefix(s.JobURL, "http") && !strings.HasPrefix(s.JobURL, "/") {
			url = careersBase + s.JobURL
		}

		results = append(results, scraper.Result{
			ID:       fmt.Sprintf("%d", s.JobCode),
			Title:    s.JobTitle,
			Company:  sourceName,
			Location: s.Location,
			Date:     createdDate,
			URL:      url,
			Metadata: map[string]string{
				"experience": s.ExperienceUI,
				"skills":     skills,
				"category":   category,
			},
		})
	}

	return results, nil
}
