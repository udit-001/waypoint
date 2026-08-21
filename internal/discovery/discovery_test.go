package discovery

import (
	"testing"
)

const fixtureCareersPage = `
<html><body>
<a href="https://job-boards.greenhouse.io/arcesium/">Open roles</a>
<a href="https://job-boards.greenhouse.io/arcesium/">dup link</a>
<a href="HTTPS://JOBS.LEVER.CO/PayTM">Lever listing</a>
<a href="https://jobs.ashbyhq.com/fampay">Ashby board</a>
<a href="https://citi.wd103.myworkdayjobs.com/CitiCareers">Workday board</a>
<a href="https://spglobal.wd3.myworkdayjobs.com/en-US/SPGI_Careers">Locale-prefixed Workday</a>
<a href="https://citi.eightfold.ai/careers">Eightfold careers</a>
<script>fetch("https://boards-api.greenhouse.io/v1/boards/other/jobs")</script>
</body></html>
`

// TestExtractBoards_findsAllVendors: one canonical URL per vendor hit,
// case-insensitive, deduplicated, Workday site slug and locale prefix
// handled.
func TestExtractBoards_findsAllVendors(t *testing.T) {
	got := ExtractBoards(fixtureCareersPage)

	byProvider := map[string][]string{}
	for _, l := range got {
		byProvider[l.Provider] = append(byProvider[l.Provider], l.URL)
	}

	want := map[string][]string{
		"greenhouse": {"https://job-boards.greenhouse.io/arcesium/"},
		"lever":      {"https://jobs.lever.co/PayTM/"},
		"ashby":      {"https://jobs.ashbyhq.com/fampay/"},
		"workday": {
			"https://citi.wd103.myworkdayjobs.com/CitiCareers/",
			"https://spglobal.wd3.myworkdayjobs.com/SPGI_Careers/", // locale prefix skipped
		},
		"eightfold": {"https://citi.eightfold.ai/careers"},
	}
	total := 0
	for provider, wantURLs := range want {
		gotURLs := byProvider[provider]
		total += len(wantURLs)
		if len(gotURLs) != len(wantURLs) {
			t.Errorf("%s: got %v, want %v", provider, gotURLs, wantURLs)
			continue
		}
		for i, u := range wantURLs {
			if gotURLs[i] != u {
				t.Errorf("%s[%d]: got %q, want %q", provider, i, gotURLs[i], u)
			}
		}
	}
	if len(got) != total {
		t.Errorf("got %d links total (%v), want %d", len(got), got, total)
	}
}

// TestExtractBoards_workdaySiteSlugCapture: the Workday regex must keep
// tenant, instance, and site slug as separate coordinates.
func TestExtractBoards_workdaySiteSlugCapture(t *testing.T) {
	cases := []struct {
		html string
		want string
	}{
		{"https://usbank.wd1.myworkdayjobs.com/US_Bank_Careers", "https://usbank.wd1.myworkdayjobs.com/US_Bank_Careers/"},
		{"https://ten.myworkdayjobs.com/wday/path", ""}, // no instance in host → not a pinnable board link
	}
	for _, tc := range cases {
		got := ExtractBoards(tc.html)
		if tc.want == "" {
			if len(got) != 0 {
				t.Errorf("ExtractBoards(%q) = %v, want none", tc.html, got)
			}
			continue
		}
		if len(got) != 1 || got[0].URL != tc.want {
			t.Errorf("ExtractBoards(%q) = %v, want [%s]", tc.html, got, tc.want)
		}
	}
}

// TestExtractBoards_emptyOnPlainPage: a careers page without ATS links
// yields nothing.
func TestExtractBoards_emptyOnPlainPage(t *testing.T) {
	html := `<html><a href="/about">About</a><p>No boards here</p></html>`
	if got := ExtractBoards(html); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}
