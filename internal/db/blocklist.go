package db

import (
	"fmt"
	"strings"
)

// The discovery blocklist: domains the user blocked after a discovery
// auto-add. Blocking is by domain (the unit discovery probes), not by
// board URL, so re-discovery skips the whole company whatever board
// link it resurfaces under. Stored as one kv key per domain — the
// BoardSweepState precedent: per-entity keys under a reserved prefix,
// no schema migration.
const kvBlockedPrefix = "discovery.blocked."

func kvBlockedKey(domain string) string { return kvBlockedPrefix + domain }

// NormalizeDomain is the blocklist's identity rule: lowercase, trimmed.
// Two spellings of one domain must block as one. Exported so other
// packages filtering by the same blocklist reuse this exact rule.
func NormalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

// BlockDomain adds a domain to the discovery blocklist. Idempotent:
// re-blocking replaces the entry.
func (s *SQLiteStore) BlockDomain(domain string) error {
	d := NormalizeDomain(domain)
	if d == "" {
		return fmt.Errorf("block domain: empty domain")
	}
	_, err := s.Exec(
		`INSERT INTO kv (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		kvBlockedKey(d), "1",
	)
	if err != nil {
		return fmt.Errorf("block domain: %w", err)
	}
	return nil
}

// BlockedDomains returns every blocked domain, normalized. An untouched
// store yields an empty list.
func (s *SQLiteStore) BlockedDomains() ([]string, error) {
	rows, err := s.Query(`SELECT key FROM kv WHERE key LIKE ?`, kvBlockedPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("load blocked domains: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan blocked domain: %w", err)
		}
		out = append(out, strings.TrimPrefix(key, kvBlockedPrefix))
	}
	return out, rows.Err()
}
