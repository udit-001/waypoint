package dates

import "testing"

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		// Empty
		{name: "empty string", raw: "", want: ""},
		{name: "whitespace only", raw: "   ", want: ""},

		// ISO YYYY-MM-DD
		{name: "ISO date", raw: "2026-07-02", want: "2026-07-02"},
		{name: "ISO date single-digit day", raw: "2026-07-05", want: "2026-07-05"},

		// DD/MM/YYYY
		{name: "DD/MM/YYYY", raw: "20/07/2026", want: "2026-07-20"},
		{name: "DD/MM/YYYY single-digit day", raw: "5/07/2026", want: "2026-07-05"},

		// DD-MM-YYYY
		{name: "DD-MM-YYYY", raw: "07-09-2022", want: "2022-09-07"},
		{name: "DD-MM-YYYY single-digit day", raw: "7-09-2022", want: "2022-09-07"},

		// Human-readable D MMMM YYYY (no comma)
		{name: "D MMMM YYYY", raw: "31 July 2026", want: "2026-07-31"},
		{name: "D MMMM YYYY single-digit day", raw: "2 July 2026", want: "2026-07-02"},

		// Human-readable D MMMM, YYYY (with comma)
		{name: "D MMMM, YYYY", raw: "15 July, 2026", want: "2026-07-15"},
		{name: "D MMMM, YYYY single-digit day", raw: "5 July, 2026", want: "2026-07-05"},

		// Human-readable MMMM D, YYYY (month-first with comma)
		{name: "MMMM D, YYYY", raw: "July 15, 2026", want: "2026-07-15"},
		{name: "MMMM D, YYYY single-digit day", raw: "July 5, 2026", want: "2026-07-05"},

		// Human-readable MMMM D YYYY (month-first no comma)
		{name: "MMMM D YYYY", raw: "July 31 2026", want: "2026-07-31"},

		// Rolling deadlines
		{name: "Open", raw: "Open", want: ""},
		{name: "Open until filled", raw: "Open until filled", want: ""},
		{name: "Rolling", raw: "Rolling", want: ""},
		{name: "open lowercase", raw: "open", want: ""},
		{name: "OPEN UNTIL FILLED uppercase", raw: "OPEN UNTIL FILLED", want: ""},
		{name: "rolling lowercase", raw: "rolling", want: ""},

		// Whitespace trimming
		{name: "leading/trailing whitespace ISO", raw: "  2026-07-02  ", want: "2026-07-02"},
		{name: "leading/trailing whitespace rolling", raw: "  Open  ", want: ""},

		// Unparseable — returned as-is
		{name: "garbage text", raw: "some random text", want: "some random text"},
		{name: "partial date", raw: "2026-07", want: "2026-07"},
		{name: "invalid month DD/MM", raw: "20/13/2026", want: "20/13/2026"},
		{name: "invalid day DD/MM", raw: "32/07/2026", want: "32/07/2026"},
		{name: "random numbers", raw: "12345", want: "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeDate(tt.raw)
			if got != tt.want {
				t.Errorf("NormalizeDate(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
