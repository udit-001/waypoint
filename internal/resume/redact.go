// Package resume extracts text from PDF resumes and redacts contact
// information before the text is passed to a model.
//
// It is a deep module: callers pass a PDF path and get safe, redacted
// text plus a structured report. Library management (download + verify),
// backend selection (pdf_oxide → poppler), and the redaction engine
// (email regex + libphonenumber-validated candidates) all live behind
// the small interface in this package.
package resume

import (
	"regexp"
	"sort"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// Options controls extraction/redaction. The only knob callers need.
type Options struct {
	// NoRedact disables redaction (raw text for the user's own eyes).
	NoRedact bool
}

// Span is one redacted region. Start and End are byte offsets into the
// original text (Go string indexing).
type Span struct {
	Type  string `json:"type"` // "email" | "phone"
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// RedactResult is what the redaction engine produces: the safe text plus
// a report the caller can verify with.
type RedactResult struct {
	Redacted      bool   `json:"redacted"`
	EmailRedacted bool   `json:"email_redacted"`
	PhoneRedacted bool   `json:"phone_redacted"`
	Spans         []Span `json:"spans"`
	Text          string `json:"text"`
}

// redactMarker replaces every redacted span.
const redactMarker = "[REDACTED]"

// defaultRegion is the libphonenumber default region for number parsing.
const defaultRegion = "US"

// emailRe covers the practical universe of resume emails; validation is
// structural (two dotted domains) and was 1.0 in earlier probes.
var emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// phoneCandidateRe locates candidate phone numbers in free text: a digit
// run with separators (plus optional extension), exactly what
// libphonenumber's reference matcher does internally. Each candidate is
// then parsed+validated by the phonenumbers library.
var phoneCandidateRe = regexp.MustCompile(`(?:\+?\d[\d\s().\-]{5,}\d)(?:\s*(?:x|ext\.?|extension)\s*\d{1,5})?`)

// Redact strips email addresses and phone numbers from text. Detection is
// deterministic: structural regex for email, libphonenumber-parsed
// candidates for phone. Protection is leniency-POSSIBLE — anything that
// parses as a plausible number is redacted, because for a redaction tool
// an honest false positive beats a silent leak.
func Redact(text string, opts Options) (RedactResult, error) {
	if opts.NoRedact {
		return RedactResult{Text: text}, nil
	}

	var spans []Span
	for _, m := range emailRe.FindAllStringIndex(text, -1) {
		spans = append(spans, Span{Type: "email", Start: m[0], End: m[1]})
	}
	for _, m := range phoneCandidateRe.FindAllStringIndex(text, -1) {
		cand := text[m[0]:m[1]]
		num, err := phonenumbers.Parse(cand, defaultRegion)
		if err != nil || !phonenumbers.IsPossibleNumber(num) {
			continue
		}
		spans = append(spans, Span{Type: "phone", Start: m[0], End: m[1]})
	}
	spans = mergeSpans(spans)

	res := RedactResult{Spans: spans, Text: text}
	for i := range spans {
		switch spans[i].Type {
		case "email":
			res.EmailRedacted = true
		case "phone":
			res.PhoneRedacted = true
		}
	}
	res.Redacted = res.EmailRedacted || res.PhoneRedacted
	if res.Redacted {
		res.Text = applySpans(text, spans)
	}
	return res, nil
}

// mergeSpans sorts spans and merges true overlaps (start < previous end).
func mergeSpans(spans []Span) []Span {
	if len(spans) < 2 {
		return spans
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
	out := []Span{spans[0]}
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s.Start < last.End {
			if s.End > last.End {
				last.End = s.End
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// applySpans rebuilds text with each span replaced by the marker.
func applySpans(text string, spans []Span) string {
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(text[last:s.Start])
		b.WriteString(redactMarker)
		last = s.End
	}
	b.WriteString(text[last:])
	return b.String()
}
