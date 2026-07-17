package cli

import "testing"

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "1.1.0", -1},
		{"1.0", "1.0.0", 0},        // missing components default to 0
		{"1.0.0", "1.0", 0},        // symmetric
		{"1.0", "1.0.1", -1},       // missing patch vs non-zero patch
		{"1.0.1", "1.0", 1},        // non-zero patch vs missing
		{"1.0.0-rc1", "1.0.0", -1}, // pre-release < release
		{"1.0.0", "1.0.0-rc1", 1},  // release > pre-release
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-rc1", "1.0.0-rc2", -1},
		{"0.9.1", "0.9.0", 1},
		{"v1.0.0", "1.0.0", 0}, // 'v' prefix handling
	}
	for _, tc := range tests {
		got := semverCompare(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("semverCompare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
