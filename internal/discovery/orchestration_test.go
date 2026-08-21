package discovery

import (
	"context"

	"strings"
	"testing"
)

// mockFetcher serves canned HTML per URL; unlisted URLs return "".
type mockFetcher map[string]string

func (m mockFetcher) Fetch(_ context.Context, url string) (string, error) {
	return m[url], nil
}

// stubVerify swaps the verification seam for a canned predicate.
func stubVerify(t *testing.T, live func(BoardLink) bool) {
	t.Helper()
	old := verifyBoard
	verifyBoard = func(_ context.Context, l BoardLink) bool { return live(l) }
	t.Cleanup(func() { verifyBoard = old })
}

const arcesiumCareers = `<a href="https://job-boards.greenhouse.io/arcesium/">Roles</a>`
const paytmCareers = `<a href="https://jobs.lever.co/paytm/">Roles</a>`

// TestDiscover_happyPath: extraction + verification produce a candidate
// carrying its facet and verified boards only.
func TestDiscover_happyPath(t *testing.T) {
	fetch := mockFetcher{"https://arcesium.com/careers": arcesiumCareers}
	stubVerify(t, func(l BoardLink) bool {
		return l.Provider == "greenhouse" // lever links die at verification
	})

	got, err := Discover(context.Background(),
		[]Facet{{Name: "fininfra", Companies: []Company{
			{Name: "Arcesium", Domain: "arcesium.com"},
			{Name: "Paytm", Domain: "paytm.com"},   // page exists, board fails verify
			{Name: "Nomura", Domain: "nomura.com"}, // no page at all
		}}},
		nil,
		Options{Fetcher: fetch})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly [Arcesium]", got)
	}
	c := got[0]
	if c.Name != "Arcesium" || c.Facet != "fininfra" || c.Domain != "arcesium.com" {
		t.Errorf("candidate = %+v", c)
	}
	if len(c.Boards) != 1 || c.Boards[0].Provider != "greenhouse" {
		t.Errorf("boards = %+v, want [greenhouse]", c.Boards)
	}
}

// TestDiscover_filtersWatched: boards whose URL is already watched are
// dropped (case- and trailing-slash-insensitive); a company left with
// nothing is not surfaced at all.
func TestDiscover_filtersWatched(t *testing.T) {
	fetch := mockFetcher{
		"https://citi.com/careers": `<a href="https://citi.wd103.myworkdayjobs.com/CitiCareers">Workday</a>` +
			arcesiumCareers,
		"https://paytm.com/careers": paytmCareers,
	}
	stubVerify(t, func(BoardLink) bool { return true })

	got, err := Discover(context.Background(),
		[]Facet{{Name: "f", Companies: []Company{
			{Name: "Citi", Domain: "citi.com"},
			{Name: "Paytm", Domain: "paytm.com"},
		}}},
		map[string]bool{
			"https://CITI.wd103.myworkdayjobs.com/CitiCareers/": true, // different case+slash
			"https://jobs.lever.co/paytm":                       true, // no trailing slash
		},
		Options{Fetcher: fetch})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Citi survives with only its unwatched greenhouse board; Paytm's
	// single board is watched → dropped entirely.
	if len(got) != 1 || got[0].Name != "Citi" {
		t.Fatalf("got %+v, want only Citi", got)
	}
	if len(got[0].Boards) != 1 || got[0].Boards[0].Provider != "greenhouse" {
		t.Errorf("Citi boards = %+v, want [greenhouse]", got[0].Boards)
	}
}

// TestDiscover_careersLadderFallback: when /careers is empty the
// careers.<domain>/ URL is tried too.
func TestDiscover_careersLadderFallback(t *testing.T) {
	fetch := mockFetcher{
		"https://fampay.in/careers":  "", // dead
		"https://careers.fampay.in/": `<a href="https://jobs.ashbyhq.com/fampay">Ashby</a>`,
		"https://toast.com/careers":  `<a href="https://jobs.lever.co/toast">Lever</a>`, // quick win stops after first
		"https://careers.toast.com/": "<!-- must not be fetched -->",
	}
	fetched := map[string]bool{}
	wrapped := fetcherFunc(func(ctx context.Context, u string) (string, error) {
		fetched[u] = true
		return fetch.Fetch(ctx, u)
	})
	stubVerify(t, func(BoardLink) bool { return true })

	got, err := Discover(context.Background(),
		[]Facet{{Name: "fintech", Companies: []Company{
			{Name: "FamPay", Domain: "fampay.in"},
			{Name: "Toast", Domain: "toast.com"},
		}}},
		nil, Options{Fetcher: wrapped})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !fetched["https://careers.fampay.in/"] {
		t.Error("second rung of the careers ladder never tried")
	}
	if fetched["https://careers.toast.com/"] {
		t.Error("ladder did not stop on quick-win provider")
	}
	if len(got) != 2 {
		t.Fatalf("got %+v, want FamPay + Toast", got)
	}
	for _, c := range got {
		if len(c.Boards) != 1 {
			t.Errorf("%s boards = %+v, want exactly one", c.Name, c.Boards)
		}
	}
}

// TestDiscover_orderDeterministic: output follows facet order then
// company name regardless of concurrency.
func TestDiscover_orderDeterministic(t *testing.T) {
	fetch := mockFetcher{}
	for _, d := range []string{"a.com", "b.com", "c.com", "d.com"} {
		fetch["https://"+d+"/careers"] = arcesiumCareers
	}
	stubVerify(t, func(BoardLink) bool { return true })

	got, err := Discover(context.Background(), []Facet{
		{Name: "z-facet", Companies: []Company{{Name: "Zeta", Domain: "d.com"}, {Name: "Alpha", Domain: "b.com"}}},
		{Name: "a-facet", Companies: []Company{{Name: "Mid", Domain: "c.com"}}},
	}, nil, Options{Fetcher: fetch, Concurrency: 8})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	var names []string
	for _, c := range got {
		names = append(names, c.Facet+"/"+c.Name)
	}
	want := "z-facet/Alpha z-facet/Zeta a-facet/Mid"
	if strings.Join(names, " ") != want {
		t.Errorf("order = %q, want %q", strings.Join(names, " "), want)
	}
}

// fetcherFunc adapts a function to scraper.Fetcher.
type fetcherFunc func(ctx context.Context, url string) (string, error)

func (f fetcherFunc) Fetch(ctx context.Context, url string) (string, error) {
	return f(ctx, url)
}
