package dates

import (
	"strings"
	"time"
)

var rollingDeadlines = map[string]bool{
	"open":              true,
	"open until filled": true,
	"rolling":           true,
}

var dateLayouts = []string{
	"2006-01-02",      // ISO: 2026-07-02
	"2/1/2006",        // DD/MM/YYYY: 20/07/2026 or 5/07/2026
	"2-1-2006",        // DD-MM-YYYY: 07-09-2022 or 7-09-2022
	"2 January 2006",  // D MMMM YYYY: 31 July 2026
	"2 January, 2006", // D MMMM, YYYY: 15 July, 2026
	"January 2, 2006", // MMMM D, YYYY: July 15, 2026
	"January 2 2006",  // MMMM D YYYY: July 31 2026
}

// NormalizeDate parses a raw date string from scraped data and returns it
// in YYYY-MM-DD format. It handles the date formats emitted by the various
// scrapers (ISO, DD/MM/YYYY, DD-MM-YYYY, human-readable with month names).
//
// Rolling deadline strings ("Open", "Open until filled", "Rolling") return "".
// Unparseable dates are returned unchanged so the caller can still reason about them.
func NormalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if rollingDeadlines[strings.ToLower(raw)] {
		return ""
	}
	for _, layout := range dateLayouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.Format("2006-01-02")
		}
	}
	return raw
}
