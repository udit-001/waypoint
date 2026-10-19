package db

import (
	"testing"
)

// TestBlockDomain_roundTrip: a blocked domain reads back; blocking is
// idempotent (re-block replaces, never accumulates); unblocked domains
// are absent.
func TestBlockDomain_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	if err := s.BlockDomain("example.com"); err != nil {
		t.Fatalf("BlockDomain: %v", err)
	}
	// Case-normalized: a re-block in any case is the same domain.
	if err := s.BlockDomain("EXAMPLE.com"); err != nil {
		t.Fatalf("BlockDomain (re-block): %v", err)
	}
	if err := s.BlockDomain("Other.Org"); err != nil {
		t.Fatalf("BlockDomain (second): %v", err)
	}

	got, err := s.BlockedDomains()
	if err != nil {
		t.Fatalf("BlockedDomains: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("BlockedDomains = %v, want 2 normalized domains", got)
	}
	seen := map[string]bool{}
	for _, d := range got {
		seen[d] = true
	}
	for _, want := range []string{"example.com", "other.org"} {
		if !seen[want] {
			t.Errorf("BlockedDomains missing %q (got %v)", want, got)
		}
	}
}

// TestBlockedDomains_empty: an untouched store yields no blocked domains
// and no error — "nothing blocked" is the zero state, not a failure.
func TestBlockedDomains_empty(t *testing.T) {
	s := sqliteStore(t)
	got, err := s.BlockedDomains()
	if err != nil {
		t.Fatalf("BlockedDomains: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("BlockedDomains = %v, want empty", got)
	}
}

// TestBlockDomain_fakeStore_parity: the FakeStore answers the same
// contract — the seam has two real adapters, not one real and one
// hypothetical.
func TestBlockDomain_fakeStore_parity(t *testing.T) {
	f := NewFakeStore()
	if err := f.BlockDomain("Example.com"); err != nil {
		t.Fatalf("BlockDomain: %v", err)
	}
	got, err := f.BlockedDomains()
	if err != nil {
		t.Fatalf("BlockedDomains: %v", err)
	}
	if len(got) != 1 || got[0] != "example.com" {
		t.Errorf("BlockedDomains = %v, want [example.com]", got)
	}
}
