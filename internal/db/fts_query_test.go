package db

import "testing"

func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"manager", "manager*"},
		{"mana", "mana*"},
		{"", ""},
		{"  ", ""},
		{"google engineer", "google* engineer*"},
		{"manager*", "manager*"}, // strips existing * before re-adding
		{"  manager  ", "manager*"},
	}
	for _, tc := range tests {
		got := buildFTSQuery(tc.input)
		if got != tc.want {
			t.Errorf("buildFTSQuery(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
