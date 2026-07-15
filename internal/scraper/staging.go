package scraper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// StagedResult is a single entry in the seen-cache file.
// It embeds the full scraped Result so the agent can reason over
// all extracted data (metadata, description, etc.) without re-fetching.
type StagedResult struct {
	FirstSeen string `json:"first_seen"`
	Status    string `json:"status"` // "new" | "dismissed"
	Result    Result `json:"result"`
}

// Staging manages the seen-cache JSON file at the given path.
// The CLI is the sole writer — deterministic, per the design decision.
type Staging struct {
	path string
	data map[string]StagedResult // keyed by URL
}

// OpenStaging loads (or creates) the staging file at path.
func OpenStaging(path string) (*Staging, error) {
	s := &Staging{
		path: path,
		data: make(map[string]StagedResult),
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil // empty staging, file created on first Add
		}
		return nil, fmt.Errorf("read staging file: %w", err)
	}

	if len(raw) == 0 {
		return s, nil
	}

	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("parse staging file: %w", err)
	}

	return s, nil
}

// IsSeen returns true if a result with this URL is already in the staging file.
func (s *Staging) IsSeen(url string) bool {
	_, ok := s.data[url]
	return ok
}

// Add writes new results to staging with status "new".
// Results already in staging are skipped (idempotent).
func (s *Staging) Add(results []Result) error {
	now := time.Now().UTC().Format("2006-01-02")
	for _, r := range results {
		if _, ok := s.data[r.URL]; ok {
			continue // already staged
		}
		s.data[r.URL] = StagedResult{
			FirstSeen: now,
			Status:    "new",
			Result:    r,
		}
	}
	return s.save()
}

// List returns staged results, optionally filtered by status.
// If status is empty, returns all entries.
func (s *Staging) List(status string) []StagedResult {
	var out []StagedResult
	for _, r := range s.data {
		if status != "" && r.Status != status {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FirstSeen > out[j].FirstSeen // newest first
	})
	return out
}

// Dismiss marks a staged result as dismissed. Idempotent.
func (s *Staging) Dismiss(url string) error {
	entry, ok := s.data[url]
	if !ok {
		return fmt.Errorf("no staged result with URL %q", url)
	}
	entry.Status = "dismissed"
	s.data[url] = entry
	return s.save()
}

// Enrich updates a staged result's Description and Metadata fields.
// Finds the entry by Result.ID (not URL). Does not overwrite search fields
// (Title, Company, Location, Date, URL). No-op if the ID isn't staged.
func (s *Staging) Enrich(id string, desc string, meta map[string]string) error {
	for url, entry := range s.data {
		if entry.Result.ID != id {
			continue
		}
		if desc != "" {
			entry.Result.Description = desc
		}
		if len(meta) > 0 {
			if entry.Result.Metadata == nil {
				entry.Result.Metadata = map[string]string{}
			}
			for k, v := range meta {
				entry.Result.Metadata[k] = v
			}
		}
		s.data[url] = entry
		return s.save()
	}
	return nil
}

// Prune removes entries older than days. Returns count removed.
func (s *Staging) Prune(days int) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	removed := 0
	for url, r := range s.data {
		if r.FirstSeen < cutoff {
			delete(s.data, url)
			removed++
		}
	}
	if removed > 0 {
		if err := s.save(); err != nil {
			return 0, err
		}
	}
	return removed, nil
}

// save writes the staging data to disk.
func (s *Staging) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal staging: %w", err)
	}
	return os.WriteFile(s.path, raw, 0644)
}
