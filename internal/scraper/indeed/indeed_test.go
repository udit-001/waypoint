package indeed

import (
	"testing"
)

const testGraphQLResponse = `{
	"data": {
		"jobSearch": {
			"pageInfo": {"nextCursor": "abc123"},
			"results": [
				{
					"job": {
						"key": "abc123key",
						"title": "Research Associate",
						"datePublished": 1753784094000,
						"description": {"html": "<p>JRF position in <b>cancer biology</b></p>"},
						"location": {
							"city": "Bengaluru",
							"admin1Code": "KA",
							"countryCode": "IN",
							"formatted": {"long": "Bengaluru, Karnataka"}
						},
						"employer": {"name": "NCBS"},
						"recruit": {"viewJobUrl": "https://indeed.com/viewjob?jk=abc123key"},
						"attributes": [
							{"key": "CF3CP", "label": "Full-time"},
							{"key": "DSQF7", "label": "Remote"}
						]
					}
				},
				{
					"job": {
						"key": "def456key",
						"title": "Project Associate",
						"datePublished": 1753784094000,
						"description": {"html": "<p>Project position</p>"},
						"location": {
							"city": "Delhi",
							"admin1Code": "DL",
							"countryCode": "IN",
							"formatted": {"long": "New Delhi, Delhi"}
						},
						"employer": {"name": "IIT Delhi"},
						"recruit": {},
						"attributes": []
					}
				}
			]
		}
	}
}`

func TestParseGraphQLResponse(t *testing.T) {
	results, cursor, err := parseResponse([]byte(testGraphQLResponse))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if cursor != "abc123" {
		t.Errorf("cursor: got %q", cursor)
	}

	r := results[0]
	if r.ID != "abc123key" {
		t.Errorf("ID: got %q", r.ID)
	}
	if r.Title != "Research Associate" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Company != "NCBS" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Location != "Bengaluru, Karnataka" {
		t.Errorf("Location: got %q", r.Location)
	}
	if r.Date != "2025-07-29" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://indeed.com/viewjob?jk=abc123key" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["attributes"] != "Full-time, Remote" {
		t.Errorf("attributes: got %q", r.Metadata["attributes"])
	}

	r2 := results[1]
	if r2.URL != "https://in.indeed.com/viewjob?jk=def456key" {
		t.Errorf("second URL (fallback): got %q", r2.URL)
	}
}

func TestStripHTML(t *testing.T) {
	expected := "Hello world\nLine 2"
	result := stripHTML("<p>Hello <b>world</b></p><p>Line 2</p>")
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
