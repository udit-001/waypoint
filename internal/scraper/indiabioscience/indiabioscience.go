package indiabioscience

import (
	"context"
	"encoding/xml"
	"html"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	feedURL    = "https://indiabioscience.org/jobs/2026/feed"
	sourceName = "IndiaBioscience"
)

// IndiaBioscience scrapes the IndiaBioscience jobs aggregator via its Atom feed.
// Covers JRF, SRF, RA, Project Associate, and faculty positions across India.
type IndiaBioscience struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(IndiaBioscience{Fetcher: &scraper.HTTPFetcher{}})
}

func (IndiaBioscience) Name() string         { return "indiabioscience" }
func (IndiaBioscience) Source() string       { return sourceName }
func (IndiaBioscience) Categories() []string { return []string{"biotech", "academic", "aggregator"} }

func (n IndiaBioscience) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	raw, err := f.Fetch(ctx, feedURL)
	if err != nil {
		return nil, err
	}

	results := parseAtomFeed(raw)

	if opts.Query != "" {
		q := strings.ToLower(opts.Query)
		filtered := results[:0]
		for _, r := range results {
			if strings.Contains(strings.ToLower(r.Title), q) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// --- Atom XML structures ---

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string       `xml:"title"`
	Links     []atomLink   `xml:"link"`
	ID        string       `xml:"id"`
	Published string       `xml:"published"`
	Updated   string       `xml:"updated"`
	Content   string       `xml:"content"`
	Categories []atomCategory `xml:"category"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

type atomCategory struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
}

var (
	tagRE   = regexp.MustCompile(`<[^>]+>`)
	spaceRE = regexp.MustCompile(`\s+`)
)

func cleanHTML(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func parseAtomFeed(raw string) []scraper.Result {
	var feed atomFeed
	if err := xml.Unmarshal([]byte(raw), &feed); err != nil {
		return nil
	}

	var results []scraper.Result
	for _, entry := range feed.Entries {
		// Get the alternate link (the job detail page URL)
		var url string
		for _, link := range entry.Links {
			if link.Rel == "alternate" {
				url = link.Href
				break
			}
		}
		if url == "" {
			continue
		}

		// Extract ID from the atom ID: tag:indiabioscience.org,...:/orgs/<org>/jobs/<slug>
		id := entry.ID
		if idx := strings.LastIndex(id, "/jobs/"); idx >= 0 {
			id = id[idx+6:]
		}

		// Parse content HTML for organization, location, deadline
		content := entry.Content
		org := extractByTagGroup(content, "h3")
		location := extractByTagGroup(content, "h4")
		deadline := extractDeadline(content)
		engagement := extractDLField(content, "Engagement")
		hours := extractDLField(content, "Hours")
		description := extractDescription(content)

		metadata := map[string]string{
			"organization": org,
			"engagement":   engagement,
			"hours":        hours,
		}

		// Add category labels
		var cats []string
		for _, c := range entry.Categories {
			if c.Label != "" {
				cats = append(cats, c.Label)
			}
		}
		if len(cats) > 0 {
			metadata["categories"] = strings.Join(cats, ", ")
		}

		results = append(results, scraper.Result{
			ID:          id,
			Title:       cleanHTML(entry.Title),
			Company:     org,
			Location:    location,
			Date:        deadline,
			URL:         url,
			Description: description,
			Metadata:    metadata,
		})
	}

	return results
}

// extractByTagGroup extracts text from the first <hgroup><hN> tag in content.
func extractByTagGroup(content, tag string) string {
	re := regexp.MustCompile(`<` + tag + `[^>]*>([\s\S]*?)</` + tag + `>`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return cleanHTML(m[1])
}

// extractDeadline extracts the deadline from <time title="15 July 2026" ...>.
func extractDeadline(content string) string {
	re := regexp.MustCompile(`<time[^>]*title="([^"]+)"`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return m[1]
}

// extractDLField extracts a <dt>Label</dt><dd>Value</dd> field from content.
func extractDLField(content, label string) string {
	re := regexp.MustCompile(`<dt>` + regexp.QuoteMeta(label) + `</dt>\s*<dd>([\s\S]*?)</dd>`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return cleanHTML(m[1])
}

// extractDescription extracts the full job description from the Atom content HTML.
// It strips the hgroup/time/dl metadata block and returns the remaining body text
// (Profile, Qualifications, Experience, How to Apply, Contact sections).
func extractDescription(content string) string {
	// Remove the hgroup, time, and dl blocks — those are parsed as separate fields
	stripped := content
	stripped = regexp.MustCompile(`<hgroup>[\s\S]*?</hgroup>`).ReplaceAllString(stripped, "")
	stripped = regexp.MustCompile(`<time[^>]*>[\s\S]*?</time>`).ReplaceAllString(stripped, "")
	stripped = regexp.MustCompile(`<dl>[\s\S]*?</dl>`).ReplaceAllString(stripped, "")
	// Convert block-level tags to newlines for readability, then strip remaining tags
	stripped = regexp.MustCompile(`</?(p|h[1-6]|li|ul|ol|div)>`).ReplaceAllString(stripped, "\n")
	stripped = tagRE.ReplaceAllString(stripped, " ")
	stripped = html.UnescapeString(stripped)
	// Clean up whitespace: collapse spaces, trim blank lines
	lines := strings.Split(stripped, "\n")
	var out []string
	for _, line := range lines {
		line = spaceRE.ReplaceAllString(line, " ")
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
