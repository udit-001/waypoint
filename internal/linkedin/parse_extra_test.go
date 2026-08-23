package linkedin

import "testing"

// Imported experience lines carry their own "- " bullets; storing them
// verbatim double-marks every line at render time (render adds "•").
func TestJoinDesc_stripsBulletMarkers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"- Team Engagement: Led initiatives", "Team Engagement: Led initiatives"},
		{"• Client Training: PPOC for clients", "Client Training: PPOC for clients"},
		{"* Project coordination: Rolling out", "Project coordination: Rolling out"},
		{"Plain sentence without marker", "Plain sentence without marker"},
	}
	for _, c := range cases {
		if got := joinDesc("", c.in); got != c.want {
			t.Errorf("joinDesc(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// Multi-line accumulation stays clean across joins.
	got := joinDesc(joinDesc("", "- First line"), "Second line")
	want := "First line\nSecond line"
	if got != want {
		t.Errorf("joined = %q, want %q", got, want)
	}
}
