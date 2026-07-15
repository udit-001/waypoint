package indiabioscience

import (
	"context"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

type mockFetcher struct {
	xml string
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) (string, error) {
	return m.xml, nil
}

const testAtomXML = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xml:lang="en">
<title>IndiaBioscience - Jobs in 2026</title>
<entry>
<title>Junior Research Fellow/Senior Research Fellow</title>
<link rel="alternate" href="https://indiabioscience.org/orgs/trf/jobs/junior-research-fellow-senior-research-fellow" type="text/html"/>
<id>tag:indiabioscience.org,2026-06-29:/orgs/trf/jobs/junior-research-fellow-senior-research-fellow</id>
<published>2026-06-29T16:26:00+05:30</published>
<updated>2026-06-29T17:22:57+05:30</updated>
<content type="html"><![CDATA[
<hgroup><h3>TRF</h3><h4>New Delhi, Delhi &amp; NCR</h4></hgroup>
<time class="red bold" title="15 July 2026" datetime="2026-07-15T00:00:00+05:30">Deadline 15 July</time>
<dl><dt>Engagement</dt><dd>Contract</dd><dt>Hours</dt><dd>Full-time</dd></dl>
<h4>Profile</h4><p>JRF position in cancer immunotherapy.</p>
]]></content>
<category term="research" label="Research"/>
<category term="masters" label="Masters"/>
<category term="delhi" label="New Delhi"/>
</entry>
<entry>
<title>Project Associate</title>
<link rel="alternate" href="https://indiabioscience.org/orgs/instem/jobs/project-associate-4" type="text/html"/>
<id>tag:indiabioscience.org,2026-06-04:/orgs/instem/jobs/project-associate-4</id>
<published>2026-06-04T16:20:00+05:30</published>
<updated>2026-06-04T16:20:00+05:30</updated>
<content type="html"><![CDATA[
<hgroup><h3>inStem</h3><h4>Bengaluru, Karnataka</h4></hgroup>
<time class="red bold" title="30 June 2026" datetime="2026-06-30T00:00:00+05:30">Deadline 30 June</time>
<dl><dt>Engagement</dt><dd>Contract</dd><dt>Hours</dt><dd>Full-time</dd></dl>
]]></content>
<category term="research" label="Research"/>
<category term="bengaluru" label="Bengaluru"/>
</entry>
</feed>`

func TestParseAtomFeed(t *testing.T) {
	results := parseAtomFeed(testAtomXML)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	r := results[0]
	if r.ID != "junior-research-fellow-senior-research-fellow" {
		t.Errorf("ID: got %q", r.ID)
	}
	if r.Title != "Junior Research Fellow/Senior Research Fellow" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Company != "TRF" {
		t.Errorf("Company: got %q", r.Company)
	}
	if r.Location != "New Delhi, Delhi & NCR" {
		t.Errorf("Location: got %q", r.Location)
	}
	if r.Date != "15 July 2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://indiabioscience.org/orgs/trf/jobs/junior-research-fellow-senior-research-fellow" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["engagement"] != "Contract" {
		t.Errorf("engagement: got %q", r.Metadata["engagement"])
	}
	if r.Metadata["hours"] != "Full-time" {
		t.Errorf("hours: got %q", r.Metadata["hours"])
	}
	if r.Metadata["categories"] != "Research, Masters, New Delhi" {
		t.Errorf("categories: got %q", r.Metadata["categories"])
	}
	if r.Description == "" {
		t.Error("Description should not be empty")
	}
	if !contains(r.Description, "JRF position in cancer immunotherapy") {
		t.Errorf("Description should contain posting text, got %q", r.Description[:min(80, len(r.Description))])
	}

	r2 := results[1]
	if r2.Company != "inStem" {
		t.Errorf("second Company: got %q", r2.Company)
	}
	if r2.Location != "Bengaluru, Karnataka" {
		t.Errorf("second Location: got %q", r2.Location)
	}
}

func TestParseAtomFeed_empty(t *testing.T) {
	results := parseAtomFeed("<feed></feed>")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestSearch_withQuery(t *testing.T) {
	n := IndiaBioscience{Fetcher: &mockFetcher{xml: testAtomXML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "research fellow"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
	if results[0].ID != "junior-research-fellow-senior-research-fellow" {
		t.Errorf("expected JRF entry, got %s", results[0].ID)
	}
}

func TestSearch_noMatch(t *testing.T) {
	n := IndiaBioscience{Fetcher: &mockFetcher{xml: testAtomXML}}

	results, err := n.Search(context.Background(), scraper.SearchOpts{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
