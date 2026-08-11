package cli

import (
	"testing"
	"time"
)

func TestParseToday(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"empty means no anchor", "", time.Time{}, false},
		{"valid date", "2026-08-12", time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC), false},
		{"bad format", "2026/08/12", time.Time{}, true},
		{"not a date", "hello", time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToday(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseToday(%q): expected error, got nil", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToday(%q): unexpected error: %v", tt.in, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseToday(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestEffectiveDate(t *testing.T) {
	anchor := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	if got := effectiveDate(anchor); !got.Equal(anchor) {
		t.Errorf("effectiveDate(anchor) = %v, want %v (explicit anchor wins)", got, anchor)
	}
	zero := effectiveDate(time.Time{})
	if zero.IsZero() {
		t.Error("effectiveDate(zero) returned zero time; want machine clock fallback")
	}
}
