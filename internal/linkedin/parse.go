package linkedin

import (
	"regexp"
	"strings"
)

// ParseProfile turns the markdown Exa's web_fetch_exa returns for a LinkedIn
// profile page into structured fields. The format (verified against live Exa
// output for public profiles) looks like:
//
//	# Bill Gates
//	URL: https://www.linkedin.com/in/williamhgates
//	Published: 2026-08-07
//
//	# Bill Gates
//
//	Chair, Gates Foundation and Founder, Breakthrough Energy
//
//	Seattle, Washington, United States (US)
//
//	8 connections • 40,553,163 followers
//
//	## Experience
//
//	### Co-chair - [Gates Foundation](https://...) (Current)
//
//	Jan 2000 - Present (26 years and 7 months)
//
//	Department: General Management • Level: Manager
//
//	### [Google](https://...)
//
//	#### Director, Google Cloud AI (Current)
//
//	Dec 2025 - Present (6 months) in Sunnyvale, California, United States
//
//	<role description>
//
//	## Education
//
//	### Masters, Computer Science by Research at [University of Warwick](https://...)
//
//	2007 - 2009 (2 years) in Coventry, United Kingdom
//
//	## Skills
//
//	.net • across • agent development • ...
//
// The parser is deliberately defensive: entries missing their required key
// (title / institution) and unparseable dates are kept with empty fields, and
// the Doc() mapper drops what the profile document would reject.
func ParseProfile(markdown string) Profile {
	ps := &profileParser{
		expClosed: map[int]bool{},
	}
	ps.run(markdown)
	return ps.p
}

type profileParser struct {
	p               Profile
	section         string
	nameSet         bool
	headlinePending bool
	curCompany      string
	expClosed       map[int]bool
}

func (ps *profileParser) run(markdown string) {
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "## "):
			ps.section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			continue
		case strings.HasPrefix(trimmed, "#### "):
			ps.handleSubHeading(trimmed)
			continue
		case strings.HasPrefix(trimmed, "### "):
			ps.handleHeading(trimmed)
			continue
		}

		if trimmed == "" {
			continue
		}
		switch ps.section {
		case "":
			ps.headerLine(trimmed)
		case "experience":
			ps.experienceLine(trimmed)
		case "education":
			ps.educationLine(trimmed)
		case "skills":
			ps.skillsLine(trimmed)
		default:
			// about, publications, honors, languages, etc. — the profile
			// document has no field for these; ignore.
		}
	}
}

// headerLine handles the pre-section block: name, headline, location, and the
// connections/followers boilerplate.
func (ps *profileParser) headerLine(trimmed string) {
	if strings.HasPrefix(trimmed, "# ") {
		if !ps.nameSet {
			ps.p.Name = cleanName(strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")))
			ps.nameSet = true
			ps.headlinePending = true
		}
		return
	}
	if strings.HasPrefix(trimmed, "URL:") || strings.HasPrefix(trimmed, "Published:") ||
		strings.HasPrefix(trimmed, "Author:") {
		return
	}
	if isConnectionsLine(trimmed) {
		return
	}
	if ps.headlinePending {
		ps.p.Headline = trimmed
		ps.headlinePending = false
		return
	}
	if ps.p.Location == "" {
		ps.p.Location = trimmed
	}
}

// handleHeading dispatches ### headings by section. In experience, a heading
// is either a role ("### Author - [O'Reilly](...) (Current)") or a company
// group ("### [Google](...)") whose roles follow as #### headings. In
// education it is an institution ("### Degree at [Institution](...)" or
// "### [Institution](...)").
func (ps *profileParser) handleHeading(trimmed string) {
	text := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
	switch ps.section {
	case "experience":
		if strings.HasPrefix(text, "[") {
			ps.curCompany = linkText(text)
			return
		}
		title, company := splitRoleHeading(text)
		if company != "" {
			ps.curCompany = company
		}
		ps.p.Exp = append(ps.p.Exp, Experience{
			Title:   stripRoleMarker(title),
			Company: ps.curCompany,
		})
	case "education":
		ps.p.Edu = append(ps.p.Edu, parseEduHeading(text))
	}
}

// handleSubHeading dispatches #### headings. In experience these are roles
// under a company group ("#### Director, Google Cloud AI (Current)"); in
// education a "#### Degree" under an institution.
func (ps *profileParser) handleSubHeading(trimmed string) {
	text := stripRoleMarker(strings.TrimSpace(strings.TrimPrefix(trimmed, "#### ")))
	switch ps.section {
	case "experience":
		ps.p.Exp = append(ps.p.Exp, Experience{Title: text, Company: ps.curCompany})
	case "education":
		if len(ps.p.Edu) > 0 {
			ps.p.Edu[len(ps.p.Edu)-1].Degree = text
		}
	}
}

// experienceLine fills the current experience entry: the first date line
// becomes start/end; following non-metadata, non-blurb lines become the
// description.
func (ps *profileParser) experienceLine(trimmed string) {
	if len(ps.p.Exp) == 0 {
		return
	}
	i := len(ps.p.Exp) - 1
	e := &ps.p.Exp[i]
	if e.Start == "" && e.End == "" {
		if start, end, ok := parseDateRange(trimmed); ok {
			e.Start, e.End = start, end
			return
		}
	}
	if strings.Contains(strings.ToLower(trimmed), "department:") ||
		strings.Contains(trimmed, "• Level:") {
		ps.expClosed[i] = true
		return
	}
	if ps.expClosed[i] || isCompanyBlurb(trimmed, e.Company) {
		return
	}
	e.Description = joinDesc(e.Description, trimmed)
}

// educationLine fills the current education entry. Lines before the date line
// are location noise (e.g. "Seattle, Washington, United States") and are
// skipped; after dates, non-blurb lines are the description.
func (ps *profileParser) educationLine(trimmed string) {
	if len(ps.p.Edu) == 0 {
		return
	}
	e := &ps.p.Edu[len(ps.p.Edu)-1]
	if e.Start == "" && e.End == "" {
		if start, end, ok := parseDateRange(trimmed); ok {
			e.Start, e.End = start, end
		}
		return // location or other pre-date noise
	}
	if isCompanyBlurb(trimmed, e.Institution) {
		return
	}
	e.Description = joinDesc(e.Description, trimmed)
}

// skillsLine collects skill names: bullet ("•") and newline separated, with
// LinkedIn boilerplate filtered out.
func (ps *profileParser) skillsLine(trimmed string) {
	for _, raw := range regexp.MustCompile(`\s*•\s*|\n+`).Split(trimmed, -1) {
		s := strings.TrimSpace(raw)
		if s == "" || isSkillNoise(s) {
			continue
		}
		dup := false
		for _, have := range ps.p.Skills {
			if strings.EqualFold(have, s) {
				dup = true
				break
			}
		}
		if !dup {
			ps.p.Skills = append(ps.p.Skills, s)
		}
	}
}

// --- helpers ---

var (
	roleMarkerRe   = regexp.MustCompile(`(?i)\s*\((current|past|present)\)\s*$`)
	markdownLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	connectionsRe  = regexp.MustCompile(`^[\d,.]+\s+(connection|connections)\b`)
	skillNoiseRe   = regexp.MustCompile(`(?i)^(show|see) all\b|^\d+\+?\s*(skills?|connections?|followers?)$`)
)

// cleanName extracts the name from the page title line, tolerating LinkedIn's
// "Name - Headline - Company | LinkedIn" title format.
func cleanName(s string) string {
	if i := strings.Index(s, " - "); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(regexp.MustCompile(`(?i)\s*\|\s*linkedin\s*$`).ReplaceAllString(s, ""))
}

// isConnectionsLine reports whether a line is LinkedIn's
// "8 connections • 40,553,163 followers" boilerplate.
func isConnectionsLine(s string) bool {
	return connectionsRe.MatchString(s)
}

// splitRoleHeading splits "Title - Company (Current)" into title and company
// parts. The title may itself contain " - " (rare), so only the first split
// separates the company.
func splitRoleHeading(text string) (title, company string) {
	if i := strings.Index(text, " - "); i >= 0 {
		return text[:i], parseCompanyPart(text[i+3:])
	}
	return text, ""
}

// parseCompanyPart extracts the company from the right side of a role heading:
// a markdown link ("[O'Reilly](https://...)") or plain text with an optional
// "(Current)"/"(Past)" marker.
func parseCompanyPart(s string) string {
	if m := markdownLinkRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return stripRoleMarker(s)
}

// stripRoleMarker removes a trailing "(Current)"/"(Past)"/"(Present)" marker.
func stripRoleMarker(s string) string {
	return strings.TrimSpace(roleMarkerRe.ReplaceAllString(s, ""))
}

// linkText extracts the display text of the first markdown link in s.
func linkText(s string) string {
	if m := markdownLinkRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(s)
}

// parseEduHeading parses "Degree at [Institution](url)", "[Institution](url)",
// or plain "Institution".
func parseEduHeading(text string) Education {
	if i := strings.Index(text, " at ["); i >= 0 {
		return Education{
			Degree:      strings.TrimSpace(text[:i]),
			Institution: linkText(text[i+len(" at "):]),
		}
	}
	if strings.HasPrefix(text, "[") {
		return Education{Institution: linkText(text)}
	}
	return Education{Institution: text}
}

// isCompanyBlurb reports whether a line is Exa's appended company/institution
// boilerplate — it starts with the entry's company/institution name (optionally
// followed by a parenthetical) and " is ".
func isCompanyBlurb(line, name string) bool {
	if name == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(name) + `(\s*\([^)]*\))?\s+is\s+`)
	return re.MatchString(line)
}

func joinDesc(existing, line string) string {
	if existing == "" {
		return line
	}
	return existing + "\n" + line
}

func isSkillNoise(s string) bool {
	return skillNoiseRe.MatchString(s)
}

// --- dates ---

var monthNums = map[string]string{
	"jan": "01", "january": "01",
	"feb": "02", "february": "02",
	"mar": "03", "march": "03",
	"apr": "04", "april": "04",
	"may": "05",
	"jun": "06", "june": "06",
	"jul": "07", "july": "07",
	"aug": "08", "august": "08",
	"sep": "09", "sept": "09", "september": "09",
	"oct": "10", "october": "10",
	"nov": "11", "november": "11",
	"dec": "12", "december": "12",
}

// dateRangeRe matches a date range at the start of a line, e.g.
// "Jan 2000 - Present (26 years and 7 months)", "1973 - 1975 (2 years) in ...",
// "Dec 2025 - Present (6 months) in Sunnyvale, ...". Separators may be
// hyphen, en dash, or em dash.
var dateRangeRe = regexp.MustCompile(`(?i)^\s*((?:[a-z]{3,9}\s+)?\d{4})\s*[-–—]\s*((?:[a-z]{3,9}\s+)?\d{4}|present)\b`)

// parseDateRange extracts a start/end pair in partial ISO form (YYYY-MM or
// YYYY; "Present" becomes empty end) from a date-range line.
func parseDateRange(line string) (start, end string, ok bool) {
	m := dateRangeRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	start, ok = convertDate(m[1])
	if !ok {
		return "", "", false
	}
	end, _ = convertDate(m[2])
	return start, end, true
}

// convertDate normalizes a date token ("Jan 2000", "2000", "Present") to
// partial ISO. Returns ok=false when the token is not a recognizable date.
func convertDate(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	if strings.EqualFold(token, "present") {
		return "", true
	}
	fields := strings.Fields(token)
	var year string
	var month string
	switch len(fields) {
	case 1:
		year = fields[0]
	case 2:
		month = fields[0]
		year = fields[1]
	default:
		return "", false
	}
	if len(year) != 4 {
		return "", false
	}
	if month != "" {
		num, ok := monthNums[strings.ToLower(month)]
		if !ok {
			return "", false
		}
		return year + "-" + num, true
	}
	return year, true
}
