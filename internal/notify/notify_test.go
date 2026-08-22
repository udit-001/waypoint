package notify

import (
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/autopilot"
)

func TestFormatShortlist(t *testing.T) {
	tests := []struct {
		name      string
		notice    autopilot.ShortlistNotice
		wantTitle string
		wantBody  []string // substrings, all must appear
	}{
		{
			name:      "one match with top pick",
			notice:    autopilot.ShortlistNotice{Count: 1, TopTitle: "Senior Go Engineer", TopCompany: "Stripe", TopScore: 88},
			wantTitle: "Waypoint · 1 new match",
			wantBody:  []string{"Top pick: Senior Go Engineer at Stripe (88)"},
		},
		{
			name:      "plural count",
			notice:    autopilot.ShortlistNotice{Count: 4, TopTitle: "Staff Engineer", TopCompany: "Databricks", TopScore: 91},
			wantTitle: "Waypoint · 4 new matches",
			wantBody:  []string{"Top pick: Staff Engineer at Databricks (91)"},
		},
		{
			name:      "no company or score",
			notice:    autopilot.ShortlistNotice{Count: 2, TopTitle: "Backend Developer"},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: Backend Developer"},
		},
		{
			name:      "long title truncated",
			notice:    autopilot.ShortlistNotice{Count: 2, TopTitle: strings.Repeat("x", 100)},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: ", "…"},
		},
		{
			name:      "empty ledger fallback copy",
			notice:    autopilot.ShortlistNotice{Count: 3},
			wantTitle: "Waypoint · 3 new matches",
			wantBody:  []string{"Review them when you have two minutes."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := formatShortlist(tt.notice)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			for _, sub := range tt.wantBody {
				if !strings.Contains(body, sub) {
					t.Errorf("body %q missing %q", body, sub)
				}
			}
			if len(body) > 200 {
				t.Errorf("body too long for a toast: %d chars", len(body))
			}
		})
	}
}
