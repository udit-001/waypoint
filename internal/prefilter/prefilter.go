// Package prefilter implements the deterministic pre-curation gate.
//
// Free Go rules act only on confident normalized matches. Everything
// else escalates to the LLM. The governing rule: wrong auto-dismiss
// is worse than one extra model call.
//
// Rules:
//   - Auto-dismiss: company in avoid_companies (exact normalized match)
//   - Auto-shortlist: company in target companies (exact normalized match)
//   - Auto-dismiss: salary below all salary floor entries (same currency)
//   - Dedup: URL already in postings or jobs
//   - Location: never a Go filter — always escalates
//   - Salary unparseable or absent: no filter, zen decides
//   - Currency mismatch: pass both numbers to zen, never convert
package prefilter

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/udit-001/waypoint/internal/scraper"
)

// Verdict is the prefilter's output for a single posting.
type Verdict struct {
	// Action is the deterministic action: "shortlist", "dismiss", or "" (escalate).
	Action string `json:"action"`
	// Reason explains why the action was taken (for logs/debug).
	Reason string `json:"reason,omitempty"`
}

// Profile holds the curation-relevant fields from the user profile.
// Shaped to match db.Profile's relevant fields — callers extract these
// from the profile before calling Filter.
type Profile struct {
	AvoidCompanies []string      // normalized company names to auto-dismiss
	Companies      []string      // normalized company names to auto-shortlist
	SalaryFloor    []SalaryFloor // per-region salary floors
}

// SalaryFloor is one region's salary floor (amount + currency code).
type SalaryFloor struct {
	Region   string `json:"region"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"` // derived from region
}

// Store is the narrow interface for dedup checks (postings + jobs).
// Matches the subset of db.Store needed by the prefilter.
type Store interface {
	HasPosting(url string) (bool, error)
	JobExists(url string) (bool, error)
}

// Filter runs the deterministic prefilter on a posting. Returns a Verdict.
// Empty Action means "escalate to zen" — the caller must not auto-act.
func Filter(p scraper.Result, profile Profile, store Store) Verdict {
	// 0. URL dedup — already tracked as a job → auto-dismiss.
	// (Posting dedup is handled at the sweep level; the prefilter
	// only checks the jobs table to avoid re-curating promoted URLs.)
	if store != nil {
		if tracked, _ := store.JobExists(p.URL); tracked {
			return Verdict{Action: "dismiss", Reason: "duplicate: URL already tracked as job"}
		}
	}

	// 1. Company match — auto-dismiss only for avoid-list.
	// Target companies are NOT auto-shortlisted: zen must evaluate each
	// posting individually (a target company can still have bad-fit roles).
	normalized := NormalizeCompany(p.Company)
	if normalized != "" {
		for _, avoid := range profile.AvoidCompanies {
			if normalized == avoid {
				return Verdict{Action: "dismiss", Reason: "avoid-list company: " + p.Company}
			}
		}
	}

	// 2. Salary floor check — only when both posting salary and floor are
	//    parseable AND the currency matches. Everything else escalates.
	if len(profile.SalaryFloor) > 0 {
		postingSalary := ExtractSalaryFromMetadata(p.Metadata)
		if postingSalary != nil {
			for _, floor := range profile.SalaryFloor {
				if floor.Currency == "" || postingSalary.Currency == "" {
					continue // unknown currency → escalate
				}
				if floor.Currency != postingSalary.Currency {
					continue // currency mismatch → escalate (zen decides)
				}
				// Compare: posting max must meet the floor minimum.
				if postingSalary.Max > 0 && postingSalary.Max < floor.Amount {
					return Verdict{
						Action: "dismiss",
						Reason: "below salary floor: " + postingSalary.Raw + " < " + floor.Currency + " " + itoa(floor.Amount),
					}
				}
			}
		}
	}

	// 3. Location, keywords, etc. — never a Go filter. Escalate.
	return Verdict{}
}

// --- Company normalizer ---

// legalSuffixes are corporate entity suffixes stripped during normalization.
// Kept conservative: "corp" and "co" are excluded because they appear
// as real parts of company names (e.g. "Evil Corp", "JP Morgan Chase Co").
var legalSuffixes = []string{
	" private limited", " pvt ltd", " incorporated", " corporation",
	" inc.", " llc", " ltd", " gmbh", " inc",
}

// punctRe matches punctuation (but not digits or spaces).
var punctRe = regexp.MustCompile(`[^\w\s]`)

// NormalizeCompany normalizes a company name for matching:
// lowercase, strip punctuation, strip legal suffixes, collapse whitespace.
// Returns "" for empty or unrecognizable input.
func NormalizeCompany(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Lowercase.
	name = strings.ToLower(name)

	// Strip punctuation (but keep digits and spaces).
	name = punctRe.ReplaceAllString(name, " ")

	// Collapse whitespace before suffix stripping so trailing spaces
	// from punctuation removal don't block HasSuffix.
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Strip legal suffixes (longest first for greedy matching).
	for _, suffix := range legalSuffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break // only strip one suffix
		}
	}

	// Final trim.
	name = strings.TrimSpace(name)

	return name
}

// --- Salary parser ---

// ParsedSalary holds the parsed min/max and currency from a posting's salary string.
type ParsedSalary struct {
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	Currency string `json:"currency"`
	Raw      string `json:"raw"`
}

// ParseSalary extracts min, max, and currency from a salary string.
// Returns nil when unparseable (caller should escalate to zen).
// Handles: "$120,000–$145,000", "₹12–18 LPA", "USD 90k/yr", "$100k-$150k",
// "₹15,00,000 - ₹20,00,000", "INR 8-12 LPA", etc.
func ParseSalary(raw string) *ParsedSalary {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Try to extract two numbers separated by a range indicator.
	// Split on range separators first, then parse each part.
	parts := splitSalaryRange(raw)
	if len(parts) < 1 {
		return nil
	}

	// Try to extract currency from the raw string.
	currency := extractSalaryCurrency(raw)

	var nums []int
	for _, part := range parts {
		n := parseSalaryPart(part)
		if n > 0 {
			nums = append(nums, n)
		}
	}

	if len(nums) == 0 {
		return nil
	}

	var min, max int
	if len(nums) == 1 {
		min = 0
		max = nums[0]
	} else {
		min = nums[0]
		max = nums[1]
		if min > max {
			min, max = max, min
		}
	}

	return &ParsedSalary{
		Min:      min,
		Max:      max,
		Currency: currency,
		Raw:      raw,
	}
}

// rangeSepRe matches range separators: en-dash, em-dash, hyphen (with or
// without spaces), and "to".
var rangeSepRe = regexp.MustCompile(`[–—]\s*|-+|\s+to\s+`)

// splitSalaryRange splits a salary string on range separators.
func splitSalaryRange(raw string) []string {
	// Split on en-dash, em-dash, hyphen(s), or " to ".
	parts := rangeSepRe.Split(raw, -1)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// salaryCurrencyRe matches currency symbols and codes at the start of a salary part.
var salaryCurrencyRe = regexp.MustCompile(`(?i)^([\$₹€£]|(?:USD|INR|EUR|GBP|SGD|AUD|CAD|JPY)\s+)`)

// extractSalaryCurrency finds the first currency token in a salary string.
func extractSalaryCurrency(raw string) string {
	m := salaryCurrencyRe.FindStringSubmatch(raw)
	if m == nil {
		return resolveCurrency("")
	}
	return resolveCurrency(m[1])
}

// salaryNumRe matches a number with optional commas and optional k/K suffix.
var salaryNumRe = regexp.MustCompile(`[\d,]+\.?\d*\s*[kK]?`)

// parseSalaryPart extracts a single number from a salary part string.
// Handles: "$120,000", "120k", "₹15,00,000", "USD 90k/yr", etc.
func parseSalaryPart(part string) int {
	m := salaryNumRe.FindString(part)
	if m == "" {
		return 0
	}

	multiplier := 1
	if strings.HasSuffix(strings.TrimSpace(m), "k") || strings.HasSuffix(strings.TrimSpace(m), "K") {
		multiplier = 1000
		m = strings.TrimSuffix(strings.TrimSuffix(m, "k"), "K")
	}

	// Remove commas and whitespace.
	m = strings.ReplaceAll(m, ",", "")
	m = strings.TrimSpace(m)

	var num float64
	for _, c := range m {
		if c >= '0' && c <= '9' {
			num = num*10 + float64(c-'0')
		} else if c == '.' {
			break
		}
	}

	return int(num) * multiplier
}

// ExtractSalaryFromMetadata tries to parse salary from posting metadata
// (key "salary") or falls back to a nil result.
func ExtractSalaryFromMetadata(meta map[string]string) *ParsedSalary {
	if meta == nil {
		return nil
	}
	if s, ok := meta["salary"]; ok {
		return ParseSalary(s)
	}
	return nil
}

// parseSalaryNum parses a number string with optional commas and k suffix.
func parseSalaryNum(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Remove commas.
	s = strings.ReplaceAll(s, ",", "")

	multiplier := 1
	if strings.HasSuffix(s, "k") || strings.HasSuffix(s, "K") {
		s = strings.TrimSuffix(strings.TrimSuffix(s, "k"), "K")
		multiplier = 1000
	}

	var num float64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			num = num*10 + float64(c-'0')
		} else if c == '.' {
			// Simple decimal handling — stop at decimal for integer result.
			break
		}
	}

	return int(num) * multiplier
}

// resolveCurrency maps a currency token to a standard code.
func resolveCurrency(token string) string {
	token = strings.TrimSpace(strings.ToUpper(token))
	switch token {
	case "$", "USD":
		return "USD"
	case "₹", "INR":
		return "INR"
	case "EUR", "€":
		return "EUR"
	case "GBP", "£":
		return "GBP"
	case "SGD":
		return "SGD"
	case "AUD":
		return "AUD"
	case "CAD":
		return "CAD"
	case "JPY", "¥":
		return "JPY"
	default:
		return ""
	}
}

// --- Helpers ---

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	// Simple int-to-string without importing strconv.
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ParseProfileFloors parses the stored salary floor JSON and returns
// SalaryFloor entries. Convenience for callers that have the raw JSON string.
func ParseProfileFloors(stored string) []SalaryFloor {
	if stored == "" || stored == "[]" {
		return nil
	}
	var entries []struct {
		Region string `json:"region"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal([]byte(stored), &entries); err != nil {
		return nil
	}
	out := make([]SalaryFloor, len(entries))
	for i, e := range entries {
		out[i] = SalaryFloor{
			Region:   e.Region,
			Amount:   e.Amount,
			Currency: deriveCurrencyFromRegion(e.Region),
		}
	}
	return out
}

// deriveCurrencyFromRegion maps a region token to a currency code.
func deriveCurrencyFromRegion(region string) string {
	key := strings.ToUpper(strings.TrimSpace(region))
	switch key {
	case "IN", "INDIA":
		return "INR"
	case "US", "USA", "UNITED STATES":
		return "USD"
	case "GB", "UK", "UNITED KINGDOM":
		return "GBP"
	case "DE", "EU", "FR", "ES", "IT", "NL", "IE":
		return "EUR"
	case "CA":
		return "CAD"
	case "AU":
		return "AUD"
	case "SG":
		return "SGD"
	case "JP":
		return "JPY"
	default:
		// Check common Indian cities (default for domestic roles).
		lower := strings.ToLower(region)
		indianCities := []string{
			"bengaluru", "bangalore", "delhi", "mumbai",
			"hyderabad", "pune", "chennai", "kolkata",
		}
		for _, city := range indianCities {
			if lower == city {
				return "INR"
			}
		}
		return ""
	}
}

// ParseProfileCompanies parses a stored JSON array string of company names
// and normalizes them for matching.
func ParseProfileCompanies(stored string) []string {
	if stored == "" || stored == "[]" {
		return nil
	}
	var names []string
	if err := json.Unmarshal([]byte(stored), &names); err != nil {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if norm := NormalizeCompany(n); norm != "" {
			out = append(out, norm)
		}
	}
	return out
}
