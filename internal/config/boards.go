package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// BoardsFile is the board store: one file, one TOML array of tables.
// Written only by the CLI (ADR 0001: CLI writes, web reads). It lives in
// data_dir — not the OS config dir — because the board list is user data
// that belongs with the database in backups, not an application setting.
type BoardsFile struct {
	Boards []BoardEntry `toml:"boards"`
}

// BoardEntry is one company ATS board. The fields are deliberately
// board-shaped, not vendor-shaped: URL + provider + flags. Vendor-specific
// coordinates (Workday tenant/site, etc.) stay inside the URL.
type BoardEntry struct {
	// Name is the short unique id for the board (lowercase, e.g. "slack").
	Name string `toml:"name"`
	// Company is the display company name.
	Company string `toml:"company"`
	// URL is the careers/board URL the provider parses.
	URL string `toml:"url"`
	// Provider is the detected provider id (greenhouse, workday, ...).
	Provider string `toml:"provider,omitempty"`
	// MaxPages caps pagination for this board (0 = provider default).
	MaxPages int `toml:"max_pages,omitempty"`
	// Enabled toggles inclusion in sweeps.
	Enabled bool `toml:"enabled"`
	// AddedAt records when the board was added (RFC3339).
	AddedAt string `toml:"added_at,omitempty"`
}

// BoardsPath returns the path to the boards file inside the data dir.
// A nil cfg falls back to the default data dir, mirroring DBPath.
func BoardsPath(cfg *Config) string {
	if cfg != nil && cfg.DataDir != "" {
		return filepath.Join(cfg.DataDir, "boards.toml")
	}
	return filepath.Join(DefaultDataDir(), "boards.toml")
}

// LoadBoards reads boards.toml from the data dir. Returns an empty store
// (not an error) if the file does not exist.
func LoadBoards(cfg *Config) (*BoardsFile, error) {
	p := BoardsPath(cfg)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &BoardsFile{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var bf BoardsFile
	if err := toml.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return &bf, nil
}

// SaveBoards writes boards.toml into the data dir, creating it if needed.
func SaveBoards(cfg *Config, bf *BoardsFile) error {
	dir := DefaultDataDir()
	if cfg != nil && cfg.DataDir != "" {
		dir = cfg.DataDir
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	p := BoardsPath(cfg)
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", p, err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(bf); err != nil {
		return fmt.Errorf("encode boards: %w", err)
	}
	return nil
}

// Find returns the board with the given name, or nil.
func (bf *BoardsFile) Find(name string) *BoardEntry {
	for i := range bf.Boards {
		if bf.Boards[i].Name == name {
			return &bf.Boards[i]
		}
	}
	return nil
}

// Upsert inserts or replaces a board by name, keeping the slice sorted by
// name for stable diffs.
func (bf *BoardsFile) Upsert(b BoardEntry) {
	for i := range bf.Boards {
		if bf.Boards[i].Name == b.Name {
			bf.Boards[i] = b
			return
		}
	}
	bf.Boards = append(bf.Boards, b)
	sort.Slice(bf.Boards, func(i, j int) bool { return bf.Boards[i].Name < bf.Boards[j].Name })
}

// Remove deletes a board by name. Returns false when it wasn't there.
func (bf *BoardsFile) Remove(name string) bool {
	for i := range bf.Boards {
		if bf.Boards[i].Name == name {
			bf.Boards = append(bf.Boards[:i], bf.Boards[i+1:]...)
			return true
		}
	}
	return false
}
