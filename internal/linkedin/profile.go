// Package linkedin fetches a public LinkedIn profile through Exa's hosted MCP
// server (the same pattern the income-tracker project uses for Groww/Kite:
// a JSON-RPC 2.0 client over Streamable HTTP in internal/mcp). The fetch uses
// Exa's web_fetch_exa tool, which renders the profile page as clean markdown;
// ParseProfile turns that markdown into the structured fields the profile
// document accepts (name, title, currentLocation, skills, experience,
// education). No API key is required — the hosted server works anonymously
// with rate limits.
//
// The parse layer is a pure function over the markdown text (ParseProfile),
// tested against fixtures modeled on real Exa output. The Fetcher wraps the
// MCP call behind a callTool seam so package tests never hit the network.
package linkedin

import "strings"

// Profile holds the structured fields extracted from a LinkedIn page.
// Field names mirror the profile document keys so the server can hand the
// doc straight to the existing PATCH validation path.
type Profile struct {
	Name     string       `json:"name"`
	Headline string       `json:"title"`
	Location string       `json:"currentLocation"`
	Skills   []string     `json:"skills"`
	Exp      []Experience `json:"experience"`
	Edu      []Education  `json:"education"`
}

// Experience is one role. Dates are partial ISO (YYYY-MM, or YYYY when the
// month is unknown); an empty End means "present" — the same convention as
// the profile document. Description carries the role's own text (achievements,
// scope) as rendered by Exa — company boilerplate blurbs are filtered out at
// parse time.
type Experience struct {
	Title       string `json:"title"`
	Company     string `json:"company"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description,omitempty"`
}

// Education is one credential. Same date convention as Experience.
type Education struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description,omitempty"`
}

// Empty reports whether nothing usable was extracted — the server returns
// 422 so the UI can say "LinkedIn may be blocking this profile" instead of
// pretending an empty prefill succeeded.
func (p Profile) Empty() bool {
	return strings.TrimSpace(p.Name) == "" &&
		strings.TrimSpace(p.Headline) == "" &&
		len(p.Skills) == 0 &&
		len(p.Exp) == 0 &&
		len(p.Edu) == 0
}

// Doc returns the profile-document keys (camelCase, matching
// PATCH /api/profile) for this profile. Entries missing their required key
// (experience title, education institution) and empty skills are dropped so
// the doc always passes the existing document validation on Apply.
func (p Profile) Doc() map[string]any {
	doc := map[string]any{}
	if v := strings.TrimSpace(p.Name); v != "" {
		doc["name"] = v
	}
	if v := strings.TrimSpace(p.Headline); v != "" {
		doc["title"] = v
	}
	if v := strings.TrimSpace(p.Location); v != "" {
		doc["currentLocation"] = v
	}
	var skills []string
	for _, s := range p.Skills {
		if v := strings.TrimSpace(s); v != "" {
			skills = append(skills, v)
		}
	}
	if len(skills) > 0 {
		doc["skills"] = skills
	}
	var exp []map[string]string
	for _, e := range p.Exp {
		if strings.TrimSpace(e.Title) == "" {
			continue
		}
		exp = append(exp, map[string]string{
			"title":       strings.TrimSpace(e.Title),
			"company":     strings.TrimSpace(e.Company),
			"start":       e.Start,
			"end":         e.End,
			"description": strings.TrimSpace(e.Description),
		})
	}
	if len(exp) > 0 {
		doc["experience"] = exp
	}
	var edu []map[string]string
	for _, e := range p.Edu {
		if strings.TrimSpace(e.Institution) == "" {
			continue
		}
		edu = append(edu, map[string]string{
			"institution": strings.TrimSpace(e.Institution),
			"degree":      strings.TrimSpace(e.Degree),
			"start":       e.Start,
			"end":         e.End,
			"description": strings.TrimSpace(e.Description),
		})
	}
	if len(edu) > 0 {
		doc["education"] = edu
	}
	return doc
}
