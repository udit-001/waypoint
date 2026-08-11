package cli

import "testing"

func TestParseListInput_CommaList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "Go,React,AWS", `["Go","React","AWS"]`},
		{"with spaces", "Go, React, AWS", `["Go","React","AWS"]`},
		{"multi-word", "Project Management, React Native, AWS", `["Project Management","React Native","AWS"]`},
		{"trailing comma", "Go,React,", `["Go","React"]`},
		{"empty items dropped", "Kubernetes,Docker,,", `["Kubernetes","Docker"]`},
		{"single", "Kubernetes", `["Kubernetes"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseListInput(tt.in)
			if err != nil {
				t.Fatalf("parseListInput(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseListInput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseListInput_JSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"array", `["Go","React","AWS"]`, `["Go","React","AWS"]`},
		{"array with spaces", `["Project Management","React Native"]`, `["Project Management","React Native"]`},
		{"comma inside an item", `["Kubernetes, Docker"]`, `["Kubernetes, Docker"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseListInput(tt.in)
			if err != nil {
				t.Fatalf("parseListInput(%q): unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseListInput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseListInput_Errors(t *testing.T) {
	for _, in := range []string{"", "   "} {
		if _, err := parseListInput(in); err == nil {
			t.Errorf("parseListInput(%q): expected error, got nil", in)
		}
	}
}
