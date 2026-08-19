package prefilter

import (
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// --- Company normalizer tests ---

func TestNormalizeCompany(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace", "  ", ""},
		{"lowercase", "Google", "google"},
		{"strip punctuation", "Burlington, Inc.", "burlington"},
		{"strip LLC", "Acme LLC", "acme"},
		{"strip Ltd", "FooBar Ltd", "foobar"},
		{"strip GmbH", "Siemens GmbH", "siemens"},
		{"strip Pvt Ltd", "Infosys Pvt Ltd", "infosys"},
		{"strip Corp kept", "Evil Corp", "evil corp"},
		{"strip Inc", "Microsoft Inc", "microsoft"},
		{"strip Incorporated", "Acme Incorporated", "acme"},
		{"collapse whitespace", "  Big   Tech  Corp  ", "big tech corp"},
		{"digits preserved", "3M", "3m"},
		{"mixed case + punct", "J.P. Morgan Chase & Co.", "j p morgan chase co"},
		{"ampersand", "AT&T", "at t"},
		{"slashes", "Accenture/Deloitte", "accenture deloitte"},
		{"ampersand stripped", "Procter & Gamble", "procter gamble"},
		{"strip dot suffix", "Acme Inc.", "acme"},
		{"single word", "Google", "google"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCompany(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeCompany(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeCompany_AmbiguousNotDismissed(t *testing.T) {
	// Governing rule: an ambiguous name must never auto-dismiss.
	// "Meta" vs "meta platforms" — partial match should NOT trigger.
	avoid := []string{"meta platforms"}
	norm := NormalizeCompany("Meta")
	for _, a := range avoid {
		if norm == a {
			t.Errorf("ambiguous company %q matched avoid-list entry %q — should escalate", norm, a)
		}
	}
}

// --- Salary parser tests ---

func TestParseSalary(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantMin int
		wantMax int
		wantCur string
		wantNil bool
	}{
		{"empty", "", 0, 0, "", true},
		{"USD range", "$120,000–$145,000", 120000, 145000, "USD", false},
		{"USD with k", "$120k-$150k", 120000, 150000, "USD", false},
		{"INR LPA", "₹12–18 LPA", 12, 18, "INR", false},
		{"INR no currency", "₹15,00,000 - ₹20,00,000", 1500000, 2000000, "INR", false},
		{"USD text prefix", "USD 90k/yr - 120k/yr", 90000, 120000, "USD", false},
		{"EUR range", "€50,000–€70,000", 50000, 70000, "EUR", false},
		{"GBP range", "£40,000 - £60,000", 40000, 60000, "GBP", false},
		{"single number", "$100,000", 0, 100000, "USD", false},
		{"no currency", "100000 - 150000", 100000, 150000, "", false},
		{"unparseable", "competitive salary", 0, 0, "", true},
		{"just text", "Negotiable", 0, 0, "", true},
		{"decimal", "$80.5k-$100k", 80000, 100000, "USD", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSalary(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseSalary(%q) = %+v, want nil", tt.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseSalary(%q) = nil, want {%d, %d, %q}", tt.in, tt.wantMin, tt.wantMax, tt.wantCur)
			}
			if got.Min != tt.wantMin {
				t.Errorf("Min = %d, want %d", got.Min, tt.wantMin)
			}
			if got.Max != tt.wantMax {
				t.Errorf("Max = %d, want %d", got.Max, tt.wantMax)
			}
			if got.Currency != tt.wantCur {
				t.Errorf("Currency = %q, want %q", got.Currency, tt.wantCur)
			}
		})
	}
}

func TestParseSalary_MinMaxSorted(t *testing.T) {
	// Ensure min ≤ max even when input is reversed.
	got := ParseSalary("$150,000–$120,000")
	if got == nil {
		t.Fatal("ParseSalary returned nil")
	}
	if got.Min != 120000 || got.Max != 150000 {
		t.Errorf("expected sorted {120000, 150000}, got {%d, %d}", got.Min, got.Max)
	}
}

func TestExtractSalaryFromMetadata(t *testing.T) {
	// With salary key.
	meta := map[string]string{"salary": "$120k-$150k"}
	got := ExtractSalaryFromMetadata(meta)
	if got == nil || got.Max != 150000 {
		t.Errorf("expected $150k max, got %v", got)
	}

	// Without salary key.
	meta = map[string]string{"department": "eng"}
	got = ExtractSalaryFromMetadata(meta)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}

	// Nil metadata.
	got = ExtractSalaryFromMetadata(nil)
	if got != nil {
		t.Errorf("expected nil for nil metadata, got %+v", got)
	}
}

// --- Rule matrix tests ---

// fakeStore is a minimal Store implementation for tests.
type fakeStore struct {
	postings map[string]bool
	jobs     map[string]bool
}

func (f *fakeStore) HasPosting(url string) (bool, error) { return f.postings[url], nil }
func (f *fakeStore) JobExists(url string) (bool, error)  { return f.jobs[url], nil }

func TestFilter_DedupJob(t *testing.T) {
	store := &fakeStore{jobs: map[string]bool{"https://example.com/b": true}}
	p := scraper.Result{URL: "https://example.com/b", Company: "Acme"}
	got := Filter(p, Profile{}, store)
	if got.Action != "dismiss" {
		t.Errorf("action = %q, want dismiss (dedup job)", got.Action)
	}
}

func TestFilter_NoDedupPosting(t *testing.T) {
	// Posting dedup is handled at the sweep level, not the prefilter.
	store := &fakeStore{postings: map[string]bool{"https://example.com/a": true}}
	p := scraper.Result{URL: "https://example.com/a", Company: "Acme"}
	got := Filter(p, Profile{}, store)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (posting dedup is sweep's job)", got.Action)
	}
}

func TestFilter_AvoidCompany(t *testing.T) {
	// Avoid list must be pre-normalized (as ParseProfileCompanies does).
	profile := Profile{
		AvoidCompanies: []string{"evil corp", "bad corp"},
	}
	tests := []struct {
		name    string
		company string
		want    string
	}{
		{"exact match", "Evil Corp", "dismiss"},
		{"case insensitive", "BAD Corp", "dismiss"},
		{"no match", "Good Corp", ""},
		{"partial no match", "Evil Corp International", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scraper.Result{URL: "https://example.com/x", Company: tt.company}
			got := Filter(p, profile, nil)
			if got.Action != tt.want {
				t.Errorf("company %q: action = %q, want %q (reason: %s)", tt.company, got.Action, tt.want, got.Reason)
			}
		})
	}
}

func TestFilter_TargetCompany(t *testing.T) {
	// Target companies are NOT auto-shortlisted — zen must evaluate each
	// posting individually. Only avoid-list triggers auto-dismiss.
	profile := Profile{
		Companies: []string{"google", "stripe"},
	}
	tests := []struct {
		company string
		want    string
	}{
		{"Google", ""}, // escalate to zen
		{"Stripe", ""}, // escalate to zen
		{"Meta", ""},   // not in any list → escalate
	}
	for _, tt := range tests {
		p := scraper.Result{URL: "https://example.com/x", Company: tt.company}
		got := Filter(p, profile, nil)
		if got.Action != tt.want {
			t.Errorf("company %q: action = %q, want %q", tt.company, got.Action, tt.want)
		}
	}
}

func TestFilter_SalaryBelowFloor(t *testing.T) {
	profile := Profile{
		SalaryFloor: []SalaryFloor{
			{Region: "IN", Amount: 1000000, Currency: "INR"},
		},
	}
	p := scraper.Result{
		URL:      "https://example.com/x",
		Company:  "Acme",
		Metadata: map[string]string{"salary": "₹8,00,000 - ₹9,00,000"},
	}
	got := Filter(p, profile, nil)
	if got.Action != "dismiss" {
		t.Errorf("action = %q, want dismiss (below floor)", got.Action)
	}
}

func TestFilter_SalaryAboveFloor(t *testing.T) {
	profile := Profile{
		SalaryFloor: []SalaryFloor{
			{Region: "IN", Amount: 1000000, Currency: "INR"},
		},
	}
	p := scraper.Result{
		URL:      "https://example.com/x",
		Company:  "Acme",
		Metadata: map[string]string{"salary": "₹15,00,000 - ₹20,00,000"},
	}
	got := Filter(p, profile, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (above floor)", got.Action)
	}
}

func TestFilter_SalaryCurrencyMismatch(t *testing.T) {
	// ₹ floor vs $ posting → escalate, never convert.
	profile := Profile{
		SalaryFloor: []SalaryFloor{
			{Region: "IN", Amount: 1000000, Currency: "INR"},
		},
	}
	p := scraper.Result{
		URL:      "https://example.com/x",
		Company:  "Acme",
		Metadata: map[string]string{"salary": "$120,000 - $150,000"},
	}
	got := Filter(p, profile, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (currency mismatch)", got.Action)
	}
}

func TestFilter_SalaryUnparseable(t *testing.T) {
	profile := Profile{
		SalaryFloor: []SalaryFloor{
			{Region: "IN", Amount: 1000000, Currency: "INR"},
		},
	}
	p := scraper.Result{
		URL:      "https://example.com/x",
		Company:  "Acme",
		Metadata: map[string]string{"salary": "competitive"},
	}
	got := Filter(p, profile, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (unparseable salary)", got.Action)
	}
}

func TestFilter_SalaryAbsent(t *testing.T) {
	profile := Profile{
		SalaryFloor: []SalaryFloor{
			{Region: "IN", Amount: 1000000, Currency: "INR"},
		},
	}
	p := scraper.Result{
		URL:     "https://example.com/x",
		Company: "Acme",
	}
	got := Filter(p, profile, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (no salary)", got.Action)
	}
}

func TestFilter_LocationNeverFiltered(t *testing.T) {
	// Location is never a Go filter — always escalates.
	profile := Profile{
		SalaryFloor: []SalaryFloor{
			{Region: "US", Amount: 200000, Currency: "USD"},
		},
	}
	p := scraper.Result{
		URL:      "https://example.com/x",
		Company:  "Acme",
		Location: "Remote, US",
		Metadata: map[string]string{"salary": "$250,000"},
	}
	got := Filter(p, profile, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (location never filtered)", got.Action)
	}
}

func TestFilter_AvoidOverridesTarget(t *testing.T) {
	// If a company is in both avoid and target, avoid wins (more conservative
	// for the user's explicit wishes).
	profile := Profile{
		AvoidCompanies: []string{"acme"},
		Companies:      []string{"acme"},
	}
	p := scraper.Result{URL: "https://example.com/x", Company: "Acme"}
	got := Filter(p, profile, nil)
	if got.Action != "dismiss" {
		t.Errorf("action = %q, want dismiss (avoid overrides target)", got.Action)
	}
}

func TestFilter_NilStoreSkipsDedup(t *testing.T) {
	p := scraper.Result{URL: "https://example.com/x", Company: "Acme"}
	got := Filter(p, Profile{}, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (nil store, no dedup)", got.Action)
	}
}

func TestFilter_AllRulesCombined(t *testing.T) {
	// Target company + above salary floor → escalate to zen (not auto-shortlist).
	profile := Profile{
		Companies: []string{"google"},
		SalaryFloor: []SalaryFloor{
			{Region: "US", Amount: 150000, Currency: "USD"},
		},
	}
	p := scraper.Result{
		URL:      "https://example.com/x",
		Company:  "Google",
		Metadata: map[string]string{"salary": "$180,000 - $220,000"},
	}
	got := Filter(p, profile, nil)
	if got.Action != "" {
		t.Errorf("action = %q, want escalate (target company goes to zen)", got.Action)
	}
}

// --- ParseProfileCompanies tests ---

func TestParseProfileCompanies(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"empty array", "[]", nil},
		{"single", `["Google"]`, []string{"google"}},
		{"multiple", `["Google", "Stripe", "  Meta  "]`, []string{"google", "stripe", "meta"}},
		{"invalid json", "not json", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseProfileCompanies(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- ParseProfileFloors tests ---

func TestParseProfileFloors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantLen int
		wantCur string
	}{
		{"empty", "", 0, ""},
		{"single", `[{"region":"IN","amount":1000000}]`, 1, "INR"},
		{"multi", `[{"region":"IN","amount":1000000},{"region":"US","amount":150000}]`, 2, ""},
		{"invalid", "not json", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseProfileFloors(tt.in)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantCur != "" && len(got) > 0 && got[0].Currency != tt.wantCur {
				t.Errorf("currency = %q, want %q", got[0].Currency, tt.wantCur)
			}
		})
	}
}
