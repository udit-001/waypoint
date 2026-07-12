package google

import (
	"testing"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

func TestParseJobArray(t *testing.T) {
	// Simulated Google Jobs nested array format
	// title=0, company=1, location=2, url=3[0][0], days_ago=12, description=19, id=28
	data := make([]interface{}, 30)
	data[0] = "Research Associate"
	data[1] = "NCBS"
	data[2] = "Bengaluru, Karnataka, India"
	data[3] = []interface{}{[]interface{}{"https://example.com/job/123"}}
	data[12] = "3 days ago"
	data[19] = "JRF position in cancer biology"
	data[28] = "job_abc123"

	result := parseJobArray(data)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Title != "Research Associate" {
		t.Errorf("Title: got %q", result.Title)
	}
	if result.Company != "NCBS" {
		t.Errorf("Company: got %q", result.Company)
	}
	if result.Location != "Bengaluru, Karnataka, India" {
		t.Errorf("Location: got %q", result.Location)
	}
	if result.URL != "https://example.com/job/123" {
		t.Errorf("URL: got %q", result.URL)
	}
	if result.Description != "JRF position in cancer biology" {
		t.Errorf("Description: got %q", result.Description)
	}
	if result.ID != "job_abc123" {
		t.Errorf("ID: got %q", result.ID)
	}
	if result.Date == "" {
		t.Error("Date should not be empty")
	}
}

func TestParseJobArray_empty(t *testing.T) {
	if parseJobArray([]interface{}{}) != nil {
		t.Error("expected nil for empty array")
	}
	if parseJobArray([]interface{}{""}) != nil {
		t.Error("expected nil for array with empty title")
	}
}

func TestParseJobArray_missingFields(t *testing.T) {
	data := make([]interface{}, 5)
	data[0] = "Scientist"

	result := parseJobArray(data)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Title != "Scientist" {
		t.Errorf("Title: got %q", result.Title)
	}
	if result.Company != "" {
		t.Errorf("Company should be empty, got %q", result.Company)
	}
	if result.URL != "" {
		t.Errorf("URL should be empty, got %q", result.URL)
	}
}

func TestParseAsyncJobs_cursor(t *testing.T) {
	body := `some_prefix data-async-fc="NEXT_CURSOR_123" some_suffix`
	_, cursor, err := parseAsyncJobs(body)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cursor != "NEXT_CURSOR_123" {
		t.Errorf("cursor: got %q", cursor)
	}
}

func TestParseAsyncJobs_empty(t *testing.T) {
	results, cursor, err := parseAsyncJobs("no json here")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}
}

func TestParseInitialJobs_empty(t *testing.T) {
	results := parseInitialJobs("<html>no jobs here</html>")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestScraperInterface(t *testing.T) {
	var s scraper.Scraper = Google{}
	if s.Name() != "google" {
		t.Errorf("Name: got %q", s.Name())
	}
	if s.Source() != "Google Jobs" {
		t.Errorf("Source: got %q", s.Source())
	}
	cats := s.Categories()
	if len(cats) == 0 {
		t.Error("Categories should not be empty")
	}
}
