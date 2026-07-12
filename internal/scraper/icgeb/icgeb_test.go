package icgeb

import (
	"testing"
)

const testHTML = `<html><body>
<article>
<div class="postdate">June 26, 2026</div>
<a href="https://www.icgeb.org/srf-vacancy/"><figure class="archive-thumbnail"><img src="thumb.jpg"/></figure></a>
<header class="entry-header">
<a href="https://www.icgeb.org/srf-vacancy/">
<h1 class="entry-title">SRF Vacancy</h1>
</a>
</header>
<div class="entry-content">
<p><b>ICGEB, New Delhi, India</b><br /> Closing date: 10 July, 2026</p>
<a href="https://www.icgeb.org/srf-vacancy/" class="more-link">Read More</a>
</div>
</article>
</body></html>`

func TestParseArticles(t *testing.T) {
	results := parseArticles(testHTML)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	r := results[0]
	if r.Title != "SRF Vacancy" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "10 July, 2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.URL != "https://www.icgeb.org/srf-vacancy/" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["post_date"] != "June 26, 2026" {
		t.Errorf("post_date: got %q", r.Metadata["post_date"])
	}
	if r.Location != "ICGEB, New Delhi, India" {
		t.Errorf("Location: got %q", r.Location)
	}
}

func TestParseArticles_empty(t *testing.T) {
	if len(parseArticles("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}
