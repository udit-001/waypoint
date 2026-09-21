package detail

import (
	"regexp"
	"strings"
)

// --- shared regex building blocks ---

var (
	salaryRe        = regexp.MustCompile(`(?:[\$₹]|Rs\.?\s?)[\d,.]+[Kk]?\s*[–—-]\s*(?:[\$₹]|Rs\.?\s?)?[\d,.]+[Kk]?(?:\s*(?:a year|/yr|per year|USD annually|annually|LPA|/hr|a month|per annum))?|[\d,.]+\s*[–—-]\s*[\d,.]+\s+(?:LPA|lakhs?|per annum|pa)`)
	salaryContextRe = regexp.MustCompile(`(?i)pay range[:\s]*([\$₹][\d,.]+[Kk]?\s*[–—-]\s[\$₹]?[\d,.]+[Kk]?[^*\n]{0,40})`)
	deadlineRe      = regexp.MustCompile(`(?i)(?:apply by|last date|application deadline|closes on|closing date)[:\s]*([^\n*]{4,60})`)
	bulletRe        = regexp.MustCompile(`^\s*[-*•]\s+`)

	// Requirement header — accepts markdown headings, bold text, and
	// mixed-case variants. Bug fix from prototype: accept **QUALIFICATIONS**.
	reqsHeaderRe = regexp.MustCompile(`(?i)^#{1,4}\s*(?:\*{0,2})(.*(?:requirement|qualification|what you.{0,3}ll (?:need|bring)|looking for someone|basic qualif).*)`)

	// Title heading — first real markdown heading that isn't boilerplate.
	// Bug fix: strip trailing # from fallback title captures.
	titleCompanyAt = regexp.MustCompile(`^# (.+)`)
	ghAtCompany    = regexp.MustCompile(`^at (.+?)(?:\[.*\])?$`)

	// Page-title fallback patterns.
	pageTitleDash = regexp.MustCompile(`^(.+) - (.+)$`)
	pageTitleAt   = regexp.MustCompile(`^(.+) @ (.+)$`)

	// Location heuristic — a city/state line near the top.
	locCityRe = regexp.MustCompile(`^([A-Z][a-zA-Z .'-]+(?:,\s*[A-Z]{2}|,\s*[A-Z][a-z]+(?:\s[A-Z][a-z]+)*)?(?:/\s*[A-Z][a-z]+)*)$`)
	// Location line must not be too long or look like a sentence.
	maxLocLen = 40

	// Stop words that should never be captured as location.
	locationStopWords = map[string]bool{
		"remote": true, "hybrid": true, "onsite": true, "on-site": true,
		"full-time": true, "part-time": true, "contract": true,
		"permanent": true, "temporary": true, "internship": true,
		"bangalore": true, "bengaluru": true, "mumbai": true,
		"delhi": true, "hyderabad": true, "pune": true,
		"san francisco": true, "new york": true, "london": true,
	}
)

// boilerplate section headings that must never become the position.
var boilerplate = map[string]bool{
	"employment type": true, "location type": true, "about the role": true,
	"about us": true, "who we are": true, "benefits": true,
	"compensation & benefits": true, "apply for this job": true,
	"interview process": true, "meet our software engineers!": true,
	"company overview": true, "the role": true, "what we offer": true,
	"our values": true, "diversity": true, "equal opportunity": true,
}

// --- Greenhouse parser ---

type greenhouseParser struct{}

func (p *greenhouseParser) Family() string { return "greenhouse" }

func (p *greenhouseParser) Parse(md string) DetailResult {
	r := DetailResult{}
	lines := strings.Split(md, "\n")

	var inReqs bool
	var reqsLines []string
	var titleSet, companySet bool
	sawTitleHeading := false

	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		if t == "" {
			continue
		}

		// Track requirements sections — accept **BOLD** headers too.
		if m := reqsHeaderRe.FindStringSubmatch(t); m != nil {
			inReqs = true
			reqsLines = nil
			continue
		}
		if inReqs && (strings.HasPrefix(t, "###") || strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "**")) {
			if strings.HasPrefix(t, "###") || strings.HasPrefix(t, "## ") {
				inReqs = false
			}
		}
		if inReqs && bulletRe.MatchString(t) {
			reqsLines = append(reqsLines, strings.TrimSpace(bulletRe.ReplaceAllString(t, "")))
		}

		// Title: first real markdown heading that isn't boilerplate.
		if !titleSet && !sawTitleHeading {
			if m := titleCompanyAt.FindStringSubmatch(t); m != nil {
				cand := strings.TrimSpace(m[1])
				// Bug fix: strip trailing # chars from title.
				cand = strings.TrimRight(cand, "# ")
				cand = strings.TrimSpace(cand)
				if !boilerplate[strings.ToLower(cand)] {
					r.Title = cand
					titleSet = true
					sawTitleHeading = true
					// Greenhouse: next "at X" line is company.
					for j := i + 1; j < min(i+4, len(lines)); j++ {
						tj := strings.TrimSpace(lines[j])
						if cm := ghAtCompany.FindStringSubmatch(tj); cm != nil && !companySet {
							r.Company = strings.TrimSpace(cm[1])
							companySet = true
						}
					}
				}
			}
		}

		// Salary: first confident match wins; "pay range" context preferred.
		if r.Salary == "" {
			if m := salaryContextRe.FindStringSubmatch(t); m != nil {
				r.Salary = strings.TrimSpace(m[1])
			} else if m := salaryRe.FindString(t); m != "" {
				r.Salary = strings.TrimSpace(m)
			}
		}

		// Deadline.
		if r.Deadline == "" {
			if m := deadlineRe.FindStringSubmatch(t); m != nil {
				r.Deadline = strings.TrimSpace(m[1])
			}
		}

		// Location: city/state line, stop-word and heuristic guarded.
		if r.Location == "" {
			lt := strings.TrimSuffix(t, "/")
			lt = strings.TrimSpace(lt)
			lower := strings.ToLower(lt)
			if len(lt) <= maxLocLen && !strings.Contains(lt, "http") && lt != r.Title &&
				!locationStopWords[lower] && !strings.HasPrefix(lt, "We're") &&
				(strings.HasPrefix(lt, "Remote") || strings.HasPrefix(lt, "Hybrid") || locCityRe.MatchString(lt)) {
				r.Location = lt
			}
		}
	}

	// Page-title fallback for title/company.
	if !titleSet || !companySet {
		for i := 0; i < min(12, len(lines)); i++ {
			t := strings.TrimSpace(lines[i])
			if !companySet {
				if m := pageTitleDash.FindStringSubmatch(t); m != nil && len(m[1]) < 40 {
					r.Company = strings.TrimSpace(m[1])
					if !titleSet {
						// Bug fix: strip trailing # from title.
						r.Title = strings.TrimSpace(strings.TrimRight(m[2], "# "))
					}
					break
				}
				if m := pageTitleAt.FindStringSubmatch(t); m != nil && len(m[1]) < 60 {
					r.Title = strings.TrimSpace(strings.TrimRight(m[1], "# "))
					r.Company = strings.TrimSpace(m[2])
					break
				}
			}
		}
	}

	if len(reqsLines) > 0 {
		r.Requirements = strings.Join(reqsLines, "\n")
	}

	return r
}

// --- Lever parser ---

type leverParser struct{}

func (p *leverParser) Family() string { return "lever" }

func (p *leverParser) Parse(md string) DetailResult {
	r := DetailResult{}
	lines := strings.Split(md, "\n")

	var inReqs bool
	var reqsLines []string
	var titleSet, companySet bool

	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		if t == "" {
			continue
		}

		// Requirements tracking.
		if m := reqsHeaderRe.FindStringSubmatch(t); m != nil {
			inReqs = true
			reqsLines = nil
			continue
		}
		if inReqs && (strings.HasPrefix(t, "###") || strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "**")) {
			if strings.HasPrefix(t, "###") || strings.HasPrefix(t, "## ") {
				inReqs = false
			}
		}
		if inReqs && bulletRe.MatchString(t) {
			reqsLines = append(reqsLines, strings.TrimSpace(bulletRe.ReplaceAllString(t, "")))
		}

		// Lever page title: "Company - Position"
		if !titleSet && !companySet {
			if m := pageTitleDash.FindStringSubmatch(t); m != nil {
				cand := strings.TrimSpace(m[1])
				if !boilerplate[strings.ToLower(cand)] && len(cand) < 40 {
					r.Company = cand
					companySet = true
					// Bug fix: strip trailing # from title.
					r.Title = strings.TrimSpace(strings.TrimRight(m[2], "# "))
					titleSet = true
				}
			}
		}

		// Title from markdown heading.
		if !titleSet {
			if m := titleCompanyAt.FindStringSubmatch(t); m != nil {
				cand := strings.TrimSpace(strings.TrimRight(m[1], "# "))
				if !boilerplate[strings.ToLower(cand)] {
					r.Title = cand
					titleSet = true
				}
			}
		}

		// Salary.
		if r.Salary == "" {
			if m := salaryContextRe.FindStringSubmatch(t); m != nil {
				r.Salary = strings.TrimSpace(m[1])
			} else if m := salaryRe.FindString(t); m != "" {
				r.Salary = strings.TrimSpace(m)
			}
		}

		// Deadline.
		if r.Deadline == "" {
			if m := deadlineRe.FindStringSubmatch(t); m != nil {
				r.Deadline = strings.TrimSpace(m[1])
			}
		}

		// Location: heuristic guarded.
		if r.Location == "" {
			lt := strings.TrimSpace(strings.TrimSuffix(t, "/"))
			lower := strings.ToLower(lt)
			// Reject job-title-like lines (contains " - " pattern) and sentences.
			if len(lt) <= maxLocLen && !strings.Contains(lt, "http") && lt != r.Title &&
				!strings.Contains(lt, " - ") && !strings.HasPrefix(lt, "We're") &&
				!locationStopWords[lower] &&
				(strings.HasPrefix(lt, "Remote") || strings.HasPrefix(lt, "Hybrid") || locCityRe.MatchString(lt)) {
				// "Remote / Hybrid" → "Remote" (first variant wins)
				if idx := strings.Index(lt, " /"); idx > 0 {
					lt = strings.TrimSpace(lt[:idx])
				}
				r.Location = lt
			}
		}
	}

	if len(reqsLines) > 0 {
		r.Requirements = strings.Join(reqsLines, "\n")
	}

	return r
}

// --- Ashby parser ---

type ashbyParser struct{}

func (p *ashbyParser) Family() string { return "ashby" }

func (p *ashbyParser) Parse(md string) DetailResult {
	r := DetailResult{}
	lines := strings.Split(md, "\n")

	var inReqs bool
	var reqsLines []string
	var titleSet, companySet bool

	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		if t == "" {
			continue
		}

		// Requirements tracking.
		if m := reqsHeaderRe.FindStringSubmatch(t); m != nil {
			inReqs = true
			reqsLines = nil
			continue
		}
		if inReqs && (strings.HasPrefix(t, "###") || strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "**")) {
			if strings.HasPrefix(t, "###") || strings.HasPrefix(t, "## ") {
				inReqs = false
			}
		}
		if inReqs && bulletRe.MatchString(t) {
			reqsLines = append(reqsLines, strings.TrimSpace(bulletRe.ReplaceAllString(t, "")))
		}

		// Ashby page title: "Position @ Company"
		if !titleSet && !companySet {
			if m := pageTitleAt.FindStringSubmatch(t); m != nil {
				cand := strings.TrimSpace(m[1])
				if !boilerplate[strings.ToLower(cand)] && len(cand) < 60 {
					r.Title = cand
					titleSet = true
					r.Company = strings.TrimSpace(m[2])
					companySet = true
				}
			}
		}

		// Title from markdown heading.
		if !titleSet {
			if m := titleCompanyAt.FindStringSubmatch(t); m != nil {
				cand := strings.TrimSpace(strings.TrimRight(m[1], "# "))
				if !boilerplate[strings.ToLower(cand)] {
					r.Title = cand
					titleSet = true
				}
			}
		}

		// Salary.
		if r.Salary == "" {
			if m := salaryContextRe.FindStringSubmatch(t); m != nil {
				r.Salary = strings.TrimSpace(m[1])
			} else if m := salaryRe.FindString(t); m != "" {
				r.Salary = strings.TrimSpace(m)
			}
		}

		// Deadline.
		if r.Deadline == "" {
			if m := deadlineRe.FindStringSubmatch(t); m != nil {
				r.Deadline = strings.TrimSpace(m[1])
			}
		}

		// Location: heuristic guarded.
		if r.Location == "" {
			lt := strings.TrimSpace(strings.TrimSuffix(t, "/"))
			lower := strings.ToLower(lt)
			// Reject job-title-like lines (contains " - " pattern) and sentences.
			if len(lt) <= maxLocLen && !strings.Contains(lt, "http") && lt != r.Title &&
				!strings.Contains(lt, " - ") && !strings.HasPrefix(lt, "We're") &&
				!locationStopWords[lower] &&
				(strings.HasPrefix(lt, "Remote") || strings.HasPrefix(lt, "Hybrid") || locCityRe.MatchString(lt)) {
				// "Remote / Hybrid" → "Remote" (first variant wins)
				if idx := strings.Index(lt, " /"); idx > 0 {
					lt = strings.TrimSpace(lt[:idx])
				}
				r.Location = lt
			}
		}
	}

	if len(reqsLines) > 0 {
		r.Requirements = strings.Join(reqsLines, "\n")
	}

	return r
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
