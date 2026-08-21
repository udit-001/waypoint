package discovery

// HardcodedFacets returns the validated company universe from the
// 2026-08-19 prototype run (57 companies, 23 mapped to live boards).
// This is the first CandidateSource adapter: plain data, one function.
// WP-152 swaps in real enumeration (zen facets + Exa company search)
// producing the same []Facet shape; nothing downstream changes.
func HardcodedFacets() []Facet {
	return []Facet{
		{Name: "ratings", Companies: []Company{
			{"Moody's", "moodys.com"},
			{"S&P Global", "spglobal.com"},
			{"Fitch Ratings", "fitchratings.com"},
			{"CRISIL", "crisil.com"},
			{"ICRA", "icra.in"},
			{"CARE Ratings", "careratings.com"},
			{"India Ratings", "indiaratings.co.in"},
			{"Acuite", "acuite.in"},
			{"Brickwork", "brickworkratings.com"},
		}},
		{Name: "market-data", Companies: []Company{
			{"FactSet", "factset.com"},
			{"MSCI", "msci.com"},
			{"LSEG", "lseg.com"},
			{"Refinitiv", "refinitiv.com"},
			{"Bloomberg", "bloomberg.com"},
			{"Dow Jones", "dowjones.com"},
			{"Morningstar", "morningstar.com"},
			{"QUODD", "quodd.com"},
			{"ISI Markets", "isimarkets.com"},
			{"CFRA", "cfraresearch.com"},
			{"New Constructs", "newconstructs.com"},
			{"FTSE Russell", "ftserussell.com"},
		}},
		{Name: "trading-research", Companies: []Company{
			{"xyt", "xyt.one"},
			{"CME Group", "cmegroup.com"},
			{"Bernstein", "bernsteinresearch.com"},
			{"BCA Research", "bcaresearch.com"},
			{"Kepler Cheuvreux", "keplercheuvreux.com"},
			{"Edison Group", "edisongroup.com"},
			{"Hedgeye", "hedgeye.com"},
		}},
		{Name: "banks", Companies: []Company{
			{"State Street", "statestreet.com"},
			{"Barclays", "barclays.com"},
			{"Nomura", "nomura.com"},
			{"Societe Generale", "societegenerale.com"},
			{"BNP Paribas", "bnpparibas.com"},
			{"Citi", "citi.com"},
			{"Morgan Stanley", "morganstanley.com"},
			{"HSBC", "hsbc.com"},
			{"JPMorgan", "jpmorgan.com"},
		}},
		{Name: "payments-fintech", Companies: []Company{
			{"PayU", "payu.in"},
			{"FamPay", "fampay.in"},
			{"Tala", "tala.com"},
			{"Branch", "branchinternational.com"},
			{"MobiKwik", "mobikwik.com"},
			{"Toast", "toast.com"},
			{"Snapmint", "snapmint.com"},
			{"SabPaisa", "sabpaisa.in"},
			{"Razorpay", "razorpay.com"},
			{"Groww", "groww.com"},
			{"Cred", "cred.club"},
			{"Paytm", "paytm.com"},
		}},
		{Name: "fininfra", Companies: []Company{
			{"Arcesium", "arcesium.com"},
			{"Vanguard", "vanguard.com"},
			{"Visa", "visa.com"},
			{"Mastercard", "mastercard.com"},
			{"US Bank", "usbank.com"},
			{"FINRA", "finra.org"},
			{"Finastra", "finastra.com"},
			{"Corpay", "corpay.com"},
		}},
	}
}
