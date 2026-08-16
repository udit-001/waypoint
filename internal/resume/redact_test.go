package resume

import (
	"strings"
	"testing"
)

// Slice 1: the redaction engine redacts an email address, reports the span.
func TestRedact_Email(t *testing.T) {
	in := "Contact Jake at jake@su.edu or call."
	got, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	if !got.Redacted {
		t.Error("Redacted = false, want true")
	}
	if !got.EmailRedacted {
		t.Error("EmailRedacted = false, want true")
	}
	if got.Text != "Contact Jake at [REDACTED] or call." {
		t.Errorf("Text = %q", got.Text)
	}
	if len(got.Spans) != 1 {
		t.Fatalf("Spans = %v, want exactly one", got.Spans)
	}
	s := got.Spans[0]
	if s.Type != "email" || s.Start != 16 || s.End != 27 {
		t.Errorf("span = %+v, want {email 16 27}", s)
	}
	if strings.Contains(got.Text, "jake@su.edu") {
		t.Error("raw email leaked into Text")
	}
}

// Slice 2: the redaction engine redacts a US phone number, reports the span.
func TestRedact_Phone(t *testing.T) {
	in := "Reach me at 512-555-1234 or email jake@su.edu"
	got, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	if !got.PhoneRedacted {
		t.Error("PhoneRedacted = false, want true")
	}
	if strings.Contains(got.Text, "512-555-1234") {
		t.Errorf("raw phone leaked: %q", got.Text)
	}
	if !got.EmailRedacted {
		t.Error("EmailRedacted = false, want true")
	}
	// both spans reported, phone first (it precedes the email in the text)
	if len(got.Spans) != 2 {
		t.Fatalf("Spans = %v, want two", got.Spans)
	}
	if got.Spans[0].Type != "phone" || got.Spans[0].Start != 12 || got.Spans[0].End != 24 {
		t.Errorf("phone span = %+v, want {phone 12 24}", got.Spans[0])
	}
}

// Slice 3: no PII present → nothing redacted, original text preserved.
func TestRedact_NoPII(t *testing.T) {
	in := "Built ordering pipelines with Go and Redis."
	got, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	if got.Redacted {
		t.Error("Redacted = true, want false")
	}
	if got.Text != in {
		t.Errorf("Text = %q, want unchanged", got.Text)
	}
	if len(got.Spans) != 0 {
		t.Errorf("Spans = %v, want none", got.Spans)
	}
}

// Slice 4: dates and years are not redacted (phonenumbers VALID rejects them).
func TestRedact_NotPhone(t *testing.T) {
	in := "GPA 3.8, graduated 2024, lived 2 years in Austin."
	got, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	if got.PhoneRedacted {
		t.Errorf("PhoneRedacted on %q: %q", in, got.Text)
	}
}

// Slice 5: overlapping spans merge into one redaction (phone inside name line etc).
func TestRedact_OverlappingSpans(t *testing.T) {
	in := "jake@su.edu 512-555-1234"
	got, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	// adjacent spans: must merge or remain separate but never overlap
	last := -1
	for _, s := range got.Spans {
		if s.Start < last {
			t.Fatalf("overlapping spans: %+v", got.Spans)
		}
		last = s.End
	}
	if strings.Contains(got.Text, "jake@su.edu") || strings.Contains(got.Text, "512-555-1234") {
		t.Errorf("PII leaked: %q", got.Text)
	}
}

// Slice 6: the real demo-resume header — hand-checked literal (email+phone
// both redacted, links untouched).
func TestRedact_JakesHeader(t *testing.T) {
	in := "Jake Ryan\n123-456-7890 | jake@su.edu | linkedin.com/in/jake | github.com/jake"
	want := "Jake Ryan\n[REDACTED] | [REDACTED] | linkedin.com/in/jake | github.com/jake"
	got, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	if got.Text != want {
		t.Errorf("Text = %q\nwant  = %q", got.Text, want)
	}
	if !got.EmailRedacted || !got.PhoneRedacted {
		t.Errorf("flags = email:%v phone:%v, want both true", got.EmailRedacted, got.PhoneRedacted)
	}
	if len(got.Spans) != 2 {
		t.Fatalf("Spans = %v, want two", got.Spans)
	}
	// byte offsets in `in`: phone at 10:22, email at 25:36 (hand-counted)
	if got.Spans[0].Type != "phone" || got.Spans[0].Start != 10 || got.Spans[0].End != 22 {
		t.Errorf("phone span = %+v, want {phone 10 22}", got.Spans[0])
	}
	if got.Spans[1].Type != "email" || got.Spans[1].Start != 25 || got.Spans[1].End != 36 {
		t.Errorf("email span = %+v, want {email 25 36}", got.Spans[1])
	}
}

// Slice 7: the "it works" check — redaction is a pure function (idempotent:
// redacting already-redacted text yields the same output, no double markers).
func TestRedact_Idempotent(t *testing.T) {
	in := "Call 512-555-1234 or jake@su.edu"
	once, err := Redact(in, Options{})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	twice, err := Redact(once.Text, Options{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if twice.Text != once.Text {
		t.Errorf("idempotency broken:\n once=%q\ntwice=%q", once.Text, twice.Text)
	}
}
