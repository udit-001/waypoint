package discovery

// HardcodedFacets returns the validated company universe from the
// 2026-08-19 prototype run (57 companies, 23 mapped to live boards).
// This is the first CandidateSource adapter: plain data, one function.
// WP-152 swaps in real enumeration (zen facets + Exa company search)
// producing the same []Facet shape; nothing downstream changes.
func HardcodedFacets() []Facet {
	return []Facet{
		{Name: "ratings", Companies: []Company{
			{Name: "Moody's", Domain: "moodys.com"},
			{Name: "S&P Global", Domain: "spglobal.com"},
			{Name: "Fitch Ratings", Domain: "fitchratings.com"},
			{Name: "CRISIL", Domain: "crisil.com"},
			{Name: "ICRA", Domain: "icra.in"},
			{Name: "CARE Ratings", Domain: "careratings.com"},
			{Name: "India Ratings", Domain: "indiaratings.co.in"},
			{Name: "Acuite", Domain: "acuite.in"},
			{Name: "Brickwork", Domain: "brickworkratings.com"},
		}},
		{Name: "market-data", Companies: []Company{
			{Name: "FactSet", Domain: "factset.com"},
			{Name: "MSCI", Domain: "msci.com"},
			{Name: "LSEG", Domain: "lseg.com"},
			{Name: "Refinitiv", Domain: "refinitiv.com"},
			{Name: "Bloomberg", Domain: "bloomberg.com"},
			{Name: "Dow Jones", Domain: "dowjones.com"},
			{Name: "Morningstar", Domain: "morningstar.com"},
			{Name: "QUODD", Domain: "quodd.com"},
			{Name: "ISI Markets", Domain: "isimarkets.com"},
			{Name: "CFRA", Domain: "cfraresearch.com"},
			{Name: "New Constructs", Domain: "newconstructs.com"},
			{Name: "FTSE Russell", Domain: "ftserussell.com"},
		}},
		{Name: "trading-research", Companies: []Company{
			{Name: "xyt", Domain: "xyt.one"},
			{Name: "CME Group", Domain: "cmegroup.com"},
			{Name: "Bernstein", Domain: "bernsteinresearch.com"},
			{Name: "BCA Research", Domain: "bcaresearch.com"},
			{Name: "Kepler Cheuvreux", Domain: "keplercheuvreux.com"},
			{Name: "Edison Group", Domain: "edisongroup.com"},
			{Name: "Hedgeye", Domain: "hedgeye.com"},
		}},
		{Name: "banks", Companies: []Company{
			{Name: "State Street", Domain: "statestreet.com"},
			{Name: "Barclays", Domain: "barclays.com"},
			{Name: "Nomura", Domain: "nomura.com"},
			{Name: "Societe Generale", Domain: "societegenerale.com"},
			{Name: "BNP Paribas", Domain: "bnpparibas.com"},
			{Name: "Citi", Domain: "citi.com"},
			{Name: "Morgan Stanley", Domain: "morganstanley.com"},
			{Name: "HSBC", Domain: "hsbc.com"},
			{Name: "JPMorgan", Domain: "jpmorgan.com"},
		}},
		{Name: "payments-fintech", Companies: []Company{
			{Name: "PayU", Domain: "payu.in"},
			{Name: "FamPay", Domain: "fampay.in"},
			{Name: "Tala", Domain: "tala.com"},
			{Name: "Branch", Domain: "branchinternational.com"},
			{Name: "MobiKwik", Domain: "mobikwik.com"},
			{Name: "Toast", Domain: "toast.com"},
			{Name: "Snapmint", Domain: "snapmint.com"},
			{Name: "SabPaisa", Domain: "sabpaisa.in"},
			{Name: "Razorpay", Domain: "razorpay.com"},
			{Name: "Groww", Domain: "groww.com"},
			{Name: "Cred", Domain: "cred.club"},
			{Name: "Paytm", Domain: "paytm.com"},
		}},
		{Name: "fininfra", Companies: []Company{
			{Name: "Arcesium", Domain: "arcesium.com"},
			{Name: "Vanguard", Domain: "vanguard.com"},
			{Name: "Visa", Domain: "visa.com"},
			{Name: "Mastercard", Domain: "mastercard.com"},
			{Name: "US Bank", Domain: "usbank.com"},
			{Name: "FINRA", Domain: "finra.org"},
			{Name: "Finastra", Domain: "finastra.com"},
			{Name: "Corpay", Domain: "corpay.com"},
		}},
	}
}
