package db

import (
	"reflect"
	"testing"
)

func TestDeriveCurrency(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{"IN", "INR"},
		{"in", "INR"},
		{"India", "INR"},
		{"GB", "GBP"},
		{"uk", "GBP"},
		{"Bengaluru", "INR"},
		{" Unknown ", ""},
		{"Mars", ""},
	}
	for _, tt := range tests {
		if got := DeriveCurrency(tt.region); got != tt.want {
			t.Errorf("DeriveCurrency(%q) = %q, want %q", tt.region, got, tt.want)
		}
	}
}

func TestParseSalaryFloor(t *testing.T) {
	t.Run("amount only defaults region", func(t *testing.T) {
		got, err := ParseSalaryFloor("100000", "IN")
		if err != nil {
			t.Fatalf("ParseSalaryFloor: %v", err)
		}
		want := []SalaryFloor{{Region: "IN", Amount: 100000}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("explicit region", func(t *testing.T) {
		got, err := ParseSalaryFloor("GB:30000", "IN")
		if err != nil {
			t.Fatalf("ParseSalaryFloor: %v", err)
		}
		want := []SalaryFloor{{Region: "GB", Amount: 30000}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("multi region", func(t *testing.T) {
		got, err := ParseSalaryFloor("IN:100000,GB:30000", "")
		if err != nil {
			t.Fatalf("ParseSalaryFloor: %v", err)
		}
		want := []SalaryFloor{{Region: "IN", Amount: 100000}, {Region: "GB", Amount: 30000}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("invalid amount", func(t *testing.T) {
		if _, err := ParseSalaryFloor("IN:abc", ""); err == nil {
			t.Error("expected error for invalid amount, got nil")
		}
	})
}

func TestSalaryFloorBrief_attachesCurrency(t *testing.T) {
	stored, err := SalaryFloorToJSON([]SalaryFloor{{Region: "IN", Amount: 100000}, {Region: "GB", Amount: 30000}})
	if err != nil {
		t.Fatalf("SalaryFloorToJSON: %v", err)
	}
	entries := salaryFloorBrief(stored)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Currency != "INR" || entries[1].Currency != "GBP" {
		t.Errorf("currencies = %q, %q; want INR, GBP", entries[0].Currency, entries[1].Currency)
	}
}

func TestDeriveSeniority(t *testing.T) {
	tests := []struct {
		name       string
		experience string
		want       string
	}{
		{"junior", `["1 year at NCBS"]`, "junior"},
		{"mid", `["5 years of research"]`, "mid"},
		{"senior", `["8 years in genomics"]`, "senior"},
		{"no parseable years", `["Research associate"]`, ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		if got := DeriveSeniority(tt.experience); got != tt.want {
			t.Errorf("%s: DeriveSeniority(%q) = %q, want %q", tt.name, tt.experience, got, tt.want)
		}
	}
}

func TestNormalizeListValues(t *testing.T) {
	got := normalizeListValues([]string{"Gojek", " gojek ", "Flipkart", "FLIPKART", ""})
	want := []string{"gojek", "flipkart"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNormalizeListJSON(t *testing.T) {
	got := normalizeListJSON(`["Gojek"," gojek ","Flipkart"]`)
	if got != `["gojek","flipkart"]` {
		t.Errorf("got %s", got)
	}
	if normalizeListJSON("") != "[]" {
		t.Errorf("empty should normalize to []")
	}
}
