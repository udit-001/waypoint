package discovery

import (
	"context"
	"errors"
	"github.com/udit-001/waypoint/internal/db"
	"strconv"
	"strings"
	"testing"
)

// fakeCompleter returns a canned expansion or error.
type fakeCompleter struct {
	out string
	err error
}

func (f fakeCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	return f.out, f.err
}

// fakeEnumerator records facets and returns canned companies per facet.
type fakeEnumerator struct {
	got   []string
	perFn func(facet string) []Company
	err   error
}

func (f *fakeEnumerator) Companies(_ context.Context, facet string) ([]Company, error) {
	f.got = append(f.got, facet)
	if f.err != nil {
		return nil, f.err
	}
	return f.perFn(facet), nil
}

const briefText = `title: Quant researcher; skills: python; keywords: markets`

func TestExpandFacets_parsesJSON(t *testing.T) {
	c := fakeCompleter{out: `["market-data","payments-fintech","ratings"]`}
	facets, err := ExpandFacets(context.Background(), briefText, c, 10)
	if err != nil {
		t.Fatalf("ExpandFacets: %v", err)
	}
	if len(facets) != 3 || facets[0] != "market-data" {
		t.Errorf("facets = %+v", facets)
	}
}

func TestExpandFacets_capsAndCleans(t *testing.T) {
	c := fakeCompleter{out: `["a","b","c","d","e"]`}
	facets, err := ExpandFacets(context.Background(), briefText, c, 3)
	if err != nil {
		t.Fatalf("ExpandFacets: %v", err)
	}
	if len(facets) != 3 {
		t.Errorf("facets = %+v, want capped at 3", facets)
	}
}

func TestExpandFacets_errorSurfaces(t *testing.T) {
	c := fakeCompleter{err: errors.New("zen: HTTP 402")}
	if _, err := ExpandFacets(context.Background(), briefText, c, 5); err == nil || !strings.Contains(err.Error(), "402") {
		t.Errorf("err = %v, want surfaced cause", err)
	}
}

func TestEnumerate_dedupsByDomainAndJoinsFacets(t *testing.T) {
	e := &fakeEnumerator{perFn: func(facet string) []Company {
		switch facet {
		case "payments":
			return []Company{{Name: "Stripe", Domain: "stripe.com"}, {Name: "Ramp", Domain: "ramp.com"}}
		case "fintech":
			return []Company{{Name: "Stripe Inc", Domain: "stripe.com"}, {Name: "Brex", Domain: "brex.com"}}
		default:
			return nil
		}
	}}

	facets, err := Enumerate(context.Background(), []string{"payments", "fintech"}, e, 10)
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	if len(facets[0].Companies) != 3 {
		t.Fatalf("companies = %+v, want 3 deduped by domain", facets[0].Companies)
	}
	var stripe *Company
	for i := range facets[0].Companies {
		if facets[0].Companies[i].Domain == "stripe.com" {
			stripe = &facets[0].Companies[i]
		}
	}
	if stripe == nil || stripe.Facets != "payments, fintech" {
		t.Errorf("stripe facets = %+v, want both facets listed", stripe)
	}
	if strings.Join(e.got, ",") != "payments,fintech" {
		t.Errorf("enumerator saw %+v", e.got)
	}
}

func TestEnumerate_boundsCalls(t *testing.T) {
	e := &fakeEnumerator{perFn: func(facet string) []Company { return nil }}
	facets := make([]string, 50)
	for i := range facets {
		facets[i] = "f" + strings.Repeat("x", i%3) + strconv.Itoa(i)
	}
	if _, err := Enumerate(context.Background(), facets, e, 12); err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	if len(e.got) != 12 {
		t.Errorf("enumerated %d facets, want capped at maxCalls=12", len(e.got))
	}
}

func TestEnumerate_rateLimitSurfacesWithHint(t *testing.T) {
	e := &fakeEnumerator{err: errors.New("exa search: tools/call returned 429: rate limited")}
	_, err := Enumerate(context.Background(), []string{"payments"}, e, 5)
	if err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "retry") {
		t.Errorf("err = %v, want rate-limit cause with retry hint", err)
	}
}

// capturingCompleter records the message pair it was handed.
type capturingCompleter struct {
	out    string
	system string
	user   string
}

func (c *capturingCompleter) Complete(_ context.Context, system, user string) (string, error) {
	c.system, c.user = system, user
	return c.out, nil
}

// TestExpandFacets_InputIsPIIFree: facet expansion is the second LLM-input
// builder in the system (curation is the other). Its only personal input is
// the rendered brief, so the never-leaves set cannot reach the wire here
// either — and a future builder that reaches for raw profile rows instead of
// the renderer fails this test.
func TestExpandFacets_InputIsPIIFree(t *testing.T) {
	f := db.NewFakeStore()
	f.Profile = db.Profile{
		Name:            "Ada Lovelace",
		Email:           "ada.lovelace@example.com",
		Phone:           "+91 98765 43210",
		CurrentLocation: "12 MG Road, Bengaluru 560001",
		VisaSponsorship: "citizen-only",
		SalaryFloor:     `[{"region":"IN","amount":4000000}]`,

		Title:    "Backend Engineer",
		Keywords: `["payments"]`,
	}
	brief, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}

	c := &capturingCompleter{out: `["payments"]`}
	if _, err := ExpandFacets(context.Background(), brief.LLMInput().Text(), c, 12); err != nil {
		t.Fatalf("ExpandFacets: %v", err)
	}

	sent := c.system + "\n" + c.user
	for _, leak := range []string{
		"Ada Lovelace", "ada.lovelace@example.com", "+91 98765 43210",
		"12 MG Road", "560001", "citizen-only", "4000000",
	} {
		if strings.Contains(sent, leak) {
			t.Errorf("PII reached the wire: %q in:\n%s", leak, sent)
		}
	}
	if !strings.Contains(sent, "Backend Engineer") {
		t.Errorf("allowlisted title missing from the facet-expansion input:\n%s", sent)
	}
}
