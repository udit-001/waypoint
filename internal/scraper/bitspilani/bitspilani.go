package bitspilani

import (
	"context"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	ajaxURL    = "https://www.bits-pilani.ac.in/wp-admin/admin-ajax.php"
	listingURL = "https://www.bits-pilani.ac.in/careers/"
	sourceName = "BITS Pilani"
)

type BITSPilani struct{}

func init() {
	scraper.Register(BITSPilani{})
}

func (BITSPilani) Name() string         { return "bitspilani" }
func (BITSPilani) Source() string       { return sourceName }
func (BITSPilani) Categories() []string { return []string{"academic", "biotech"} }

func (n BITSPilani) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	body, err := fetchAJAX(ctx)
	if err != nil {
		return nil, err
	}

	results := parseTable(body)

	return results, nil
}

func fetchAJAX(ctx context.Context) (string, error) {
	payload := "action=fetch_current_position&campus=&department=&type=&campus_type=&month=&year=&search=&post_id=&faculty=&paged=1&page_name=non-academic&current_page_id=18176"

	const maxRetries = 6
	delay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ajaxURL, strings.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Referer", listingURL)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt == maxRetries {
				return "", fmt.Errorf("request failed: %d", resp.StatusCode)
			}
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			select {
			case <-time.After(delay + jitter):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			delay *= 2
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			return "", fmt.Errorf("request failed: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	return "", fmt.Errorf("request failed after max retries")
}

var (
	tagRE   = regexp.MustCompile(`<[^>]+>`)
	spaceRE = regexp.MustCompile(`\s+`)
)

func clean(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func parseTable(body string) []scraper.Result {
	var results []scraper.Result

	rowRE := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	tdRE := regexp.MustCompile(`<td[^>]*>([\s\S]*?)</td>`)
	anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)

	for _, m := range rowRE.FindAllStringSubmatch(body, -1) {
		row := m[1]
		if !strings.Contains(row, "<td") || strings.Contains(row, "<th") {
			continue
		}

		cells := tdRE.FindAllStringSubmatch(row, -1)
		if len(cells) < 5 {
			continue
		}

		titleCell := cells[1][1]
		deadline := clean(cells[4][1])
		campus := clean(cells[2][1])

		var detailURL, pdfURL string
		for _, cell := range cells[5:] {
			anchor := anchorRE.FindStringSubmatch(cell[1])
			if anchor == nil {
				continue
			}
			href := anchor[1]
			if strings.HasSuffix(href, ".pdf") && pdfURL == "" {
				pdfURL = href
			} else if detailURL == "" {
				detailURL = href
			}
		}

		title := clean(titleCell)
		if title == "" || title == "New" {
			continue
		}

		id := detailURL
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			id = strings.TrimSuffix(id[idx+1:], "/")
		}

		url := detailURL
		if url == "" {
			url = pdfURL
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: campus,
			Date:     deadline,
			URL:      url,
			Metadata: map[string]string{
				"campus":  campus,
				"pdf_url": pdfURL,
			},
		})
	}

	return results
}
