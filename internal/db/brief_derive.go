package db

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// SalaryFloor is one region's salary floor. Currency is never stored; it is
// derived from Region at read time.
type SalaryFloor struct {
	Region string `json:"region"`
	Amount int    `json:"amount"`
}

// SalaryFloorEntry is the JSON shape used for storage and the brief readout.
type SalaryFloorEntry struct {
	Region string `json:"region"`
	Amount int    `json:"amount"`
	// Currency is derived from Region, never persisted. It is present ONLY in
	// the brief readout (GetBrief), not in the stored profile.
	Currency string `json:"currency,omitempty"`
}

// countryCurrencies maps region tokens (ISO code, common name, or well-known
// city) to their currency code. Derivation is a fact — the user is never
// asked for a currency. Unknown regions derive an empty currency.
var countryCurrencies = map[string]string{
	// ISO 3166-1 alpha-2
	"IN": "INR", "US": "USD", "GB": "GBP", "UK": "GBP", "EU": "EUR",
	"DE": "EUR", "FR": "EUR", "ES": "EUR", "IT": "EUR", "NL": "EUR", "IE": "EUR",
	"CA": "CAD", "AU": "AUD", "SG": "SGD", "AE": "AED", "JP": "JPY",
	// Common names
	"India": "INR", "United States": "USD", "USA": "USD", "United Kingdom": "GBP",
	"Germany": "EUR", "France": "EUR", "Singapore": "SGD", "Australia": "AUD",
	"Canada": "CAD", "Japan": "JPY", "United Arab Emirates": "AED",
	// Well-known Indian cities (default currency for domestic roles)
	"Bengaluru": "INR", "Bangalore": "INR", "Delhi": "INR", "Mumbai": "INR",
	"Hyderabad": "INR", "Pune": "INR", "Chennai": "INR", "Kolkata": "INR",
	// London as the common UK hunt target
	"London": "GBP",
}

// DeriveCurrency returns the currency code for a region token, or "" for an
// unknown region. Case-folded and trimmed.
func DeriveCurrency(region string) string {
	key := strings.ToLower(strings.TrimSpace(region))
	for token, cur := range countryCurrencies {
		if strings.ToLower(token) == key {
			return cur
		}
	}
	return ""
}

// parseSalaryFloor parses a raw salary-floor token. Supported shapes:
//
//	"100000"      → amount only, region defaults to defaultRegion
//	"IN:100000"   → explicit region:amount
//	"IN:100000,GB:30000" → multiple region:amount pairs (comma-separated)
func ParseSalaryFloor(raw, defaultRegion string) ([]SalaryFloor, error) {
	raw = strings.TrimSpace(raw)
	var out []SalaryFloor
	if raw == "" {
		return out, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		region := defaultRegion
		amountStr := part
		if i := strings.Index(part, ":"); i >= 0 {
			region = strings.TrimSpace(part[:i])
			amountStr = strings.TrimSpace(part[i+1:])
		}
		var amount int
		if _, err := fmt.Sscanf(amountStr, "%d", &amount); err != nil {
			return nil, fmt.Errorf("invalid salary amount %q", amountStr)
		}
		out = append(out, SalaryFloor{Region: region, Amount: amount})
	}
	return out, nil
}

// salaryFloorToJSON serializes a list of salary floors to the stored JSON
// array. Empty list → "[]".
func SalaryFloorToJSON(floors []SalaryFloor) (string, error) {
	entries := make([]SalaryFloorEntry, 0, len(floors))
	for _, f := range floors {
		// Currency is deliberately NOT persisted.
		entries = append(entries, SalaryFloorEntry{Region: f.Region, Amount: f.Amount})
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// salaryFloorBrief parses the stored JSON array and attaches the derived
// currency to each entry for the brief readout.
func salaryFloorBrief(stored string) []SalaryFloorEntry {
	if stored == "" || stored == "[]" {
		return nil
	}
	var entries []SalaryFloorEntry
	if err := json.Unmarshal([]byte(stored), &entries); err != nil {
		return nil
	}
	for i := range entries {
		entries[i].Currency = DeriveCurrency(entries[i].Region)
	}
	return entries
}

// experienceYearsRe matches a "N years|yrs|yr" figure in free text. It is the
// fallback for experience entries without structured dates (see yearsInText).
var experienceYearsRe = regexp.MustCompile(`(?i)(\d+)\s*y(?:ea)?rs?`)

// DeriveSeniority maps total experience to a level. Returns "" when experience
// carries no year signal. Structured entries are counted by date range; entries
// without dates fall back to a regex on their text.
func DeriveSeniority(experience string) string {
	total := experienceYears(experience)
	switch {
	case total == 0:
		return ""
	case total < 3:
		return "junior"
	case total < 6:
		return "mid"
	default:
		return "senior"
	}
}

// normalizeListValues lowercases, trims, and de-duplicates a slice of strings
// while preserving order. Used to normalize stored list preferences for
// matching.
func normalizeListValues(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// normalizeListJSON re-serializes a stored JSON-array string with values
// normalized (case-fold, trim, dedupe). Unparseable/empty input → "[]".
func normalizeListJSON(stored string) string {
	values := stringList(stored)
	values = normalizeListValues(values)
	b, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(b)
}
