package db

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ExperienceEntry is one structured role on a resume. Dates are partial ISO
// (YYYY-MM, or YYYY when the month is unknown); an empty End means "present".
// Description carries free-text detail about the role (bullet points, scope).
type ExperienceEntry struct {
	Title       string `json:"title"`
	Company     string `json:"company"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description,omitempty"`
}

// EducationEntry is one structured credential. Same date convention as
// ExperienceEntry; Description carries free-text detail (GPA, focus areas).
type EducationEntry struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description,omitempty"`
}

// ParseExperienceEntries parses the stored JSON-array string into structured
// entries. Legacy flat-string arrays (V6 and earlier) are upgraded on read:
// each plain string becomes an entry with the text as its title and empty
// company/dates, so old rows never break and the user re-fills the fields.
func ParseExperienceEntries(stored string) []ExperienceEntry {
	return parseEntries(stored, func(s string) ExperienceEntry {
		return ExperienceEntry{Title: s}
	})
}

// ParseEducationEntries is the education counterpart of ParseExperienceEntries.
func ParseEducationEntries(stored string) []EducationEntry {
	return parseEntries(stored, func(s string) EducationEntry {
		return EducationEntry{Institution: s}
	})
}

// parseEntries is the shared upgrade-on-read for object arrays. It unmarshals
// the stored array; a plain-string item is wrapped via wrap, an object item is
// kept, and anything else is skipped.
func parseEntries[T any](stored string, wrap func(s string) T) []T {
	if stored == "" || stored == "[]" {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(stored), &raw); err != nil {
		return nil
	}
	out := make([]T, 0, len(raw))
	for _, item := range raw {
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			if strings.TrimSpace(s) != "" {
				out = append(out, wrap(s))
			}
			continue
		}
		var t T
		if err := json.Unmarshal(item, &t); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// experienceToJSON serializes structured entries to the stored JSON array.
// Prefer SerializeExperience at the write seam — it validates before
// serializing; this is the marshal-only helper used internally and by tests.
func experienceToJSON(entries []ExperienceEntry) (string, error) {
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// educationToJSON serializes structured entries to the stored JSON array.
func educationToJSON(entries []EducationEntry) (string, error) {
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SerializeExperience is the write seam for experience: parse a client-supplied
// JSON array of {title, company, start, end} objects, validate each entry, and
// return the stored JSON-array string. Shared by the CLI and the web route so
// the entry rule lives in one place.
func SerializeExperience(raw []byte) (string, error) {
	var entries []ExperienceEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", fmt.Errorf("must be a JSON array of {title, company, start, end, description} objects")
	}
	for i, e := range entries {
		if err := validateExperienceEntry(e); err != nil {
			return "", fmt.Errorf("entry %d: %w", i+1, err)
		}
	}
	return experienceToJSON(entries)
}

// SerializeEducation is the education counterpart of SerializeExperience.
func SerializeEducation(raw []byte) (string, error) {
	var entries []EducationEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", fmt.Errorf("must be a JSON array of {institution, degree, start, end, description} objects")
	}
	for i, e := range entries {
		if err := validateEducationEntry(e); err != nil {
			return "", fmt.Errorf("entry %d: %w", i+1, err)
		}
	}
	return educationToJSON(entries)
}

// validateExperienceEntry enforces the experience entry rule: a title, and
// partial dates when present.
func validateExperienceEntry(e ExperienceEntry) error {
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if err := validatePartialDate(e.Start); err != nil {
		return err
	}
	return validatePartialDate(e.End)
}

// validateEducationEntry enforces the education entry rule: an institution, and
// partial dates when present.
func validateEducationEntry(e EducationEntry) error {
	if strings.TrimSpace(e.Institution) == "" {
		return fmt.Errorf("institution is required")
	}
	if err := validatePartialDate(e.Start); err != nil {
		return err
	}
	return validatePartialDate(e.End)
}

// partialDateMonths returns a partial ISO date (YYYY-MM or YYYY) as a count of
// months since year zero, or -1 when unparseable. A year-only date counts from
// January of that year. A malformed month form is rejected outright rather than
// falling back to the year prefix.
func partialDateMonths(s string) int {
	s = strings.TrimSpace(s)
	if len(s) == 7 && s[4] == '-' {
		y, e1 := strconv.Atoi(s[:4])
		m, e2 := strconv.Atoi(s[5:7])
		if e1 == nil && e2 == nil && y >= 0 && m >= 1 && m <= 12 {
			return y*12 + (m - 1)
		}
		return -1
	}
	if len(s) == 4 {
		y, err := strconv.Atoi(s)
		if err == nil && y >= 0 {
			return y * 12
		}
	}
	return -1
}

// entryMonths returns the number of months spanned by an entry, or 0 when the
// entry has no parseable start date. An empty end ("present") counts through
// the current month.
func entryMonths(start, end string) int {
	startM := partialDateMonths(start)
	if startM < 0 {
		return 0
	}
	endM := partialDateMonths(end)
	if endM < 0 {
		now := time.Now()
		endM = now.Year()*12 + int(now.Month()) - 1
	}
	if endM < startM {
		return 0
	}
	return endM - startM
}

// experienceYears totalises the years an experience list spans: structured
// entries contribute their date ranges; entries without dates fall back to the
// regex on their text. Used by DeriveSeniority.
func experienceYears(experience string) float64 {
	entries := ParseExperienceEntries(experience)
	var total float64
	dateCount := 0
	for _, e := range entries {
		if months := entryMonths(e.Start, e.End); months > 0 {
			total += float64(months) / 12
			dateCount++
		} else if y := yearsInText(e.Title + " " + e.Company + " " + e.Description); y > 0 {
			total += float64(y)
		}
	}
	if dateCount == 0 && total == 0 {
		// No structured dates at all — fall back to the legacy flat-list scan.
		for _, s := range stringList(experience) {
			if y := yearsInText(s); float64(y) > total {
				total = float64(y)
			}
		}
	}
	return total
}

// yearsInText pulls the first parseable "N years|yrs|yr" figure from a string.
func yearsInText(s string) int {
	m := experienceYearsRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// validatePartialDate reports whether s is a valid partial ISO date (YYYY-MM or
// YYYY) or empty (empty is valid — it means "present" for an end date).
func validatePartialDate(s string) error {
	if s == "" {
		return nil
	}
	if partialDateMonths(s) < 0 {
		return fmt.Errorf("invalid date %q — use YYYY-MM or YYYY", s)
	}
	return nil
}
