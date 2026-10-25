package notify

import (
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/autopilot"
)

func TestFormatShortlist(t *testing.T) {
	tests := []struct {
		name      string
		events    []autopilot.ShortlistEvent
		wantTitle string
		wantBody  []string // substrings, all must appear
	}{
		{
			name:      "one match with top pick",
			events:    []autopilot.ShortlistEvent{{Title: "Senior Go Engineer", Company: "Stripe", Score: 88}},
			wantTitle: "Waypoint · 1 new match",
			wantBody:  []string{"Top pick: Senior Go Engineer at Stripe (88)"},
		},
		{
			name: "plural count, strongest wins",
			events: []autopilot.ShortlistEvent{
				{Title: "Staff Engineer", Company: "Databricks", Score: 91},
				{Title: "Backend Developer", Score: 62},
			},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: Staff Engineer at Databricks (91)"},
		},
		{
			name: "no company or score",
			events: []autopilot.ShortlistEvent{
				{Title: "Backend Developer"},
				{Title: "Another Role"},
			},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: Backend Developer"},
		},
		{
			name: "long title truncated",
			events: []autopilot.ShortlistEvent{
				{Title: strings.Repeat("x", 100)},
				{Title: "y"},
			},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: ", "…"},
		},
		{
			name: "all scores zero picks the first event",
			events: []autopilot.ShortlistEvent{
				{Title: "First In", Company: "FirstCo"},
				{Title: "Second In", Company: "SecondCo"},
			},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: First In at FirstCo"},
		},
		{
			name: "unscored event first still loses to a scored one",
			events: []autopilot.ShortlistEvent{
				{Title: "Prefilter Pick", Company: "FirstCo"},
				{Title: "Scored Pick", Company: "HiCo", Score: 92},
			},
			wantTitle: "Waypoint · 2 new matches",
			wantBody:  []string{"Top pick: Scored Pick at HiCo (92)"},
		},
		{
			name:      "empty batch fallback copy",
			events:    nil,
			wantTitle: "Waypoint · 0 new matches",
			wantBody:  []string{"Review them when you have two minutes."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := formatShortlist(tt.events)
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
