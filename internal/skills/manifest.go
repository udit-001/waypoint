package skills

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Manifest struct {
	Hash  string   `json:"hash"`
	Files []string `json:"files"`
}

func ManifestPath(dir string) string {
	return filepath.Join(dir, "waypoint.skill.json")
}

func ManifestHash(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(files[p])
		h.Write([]byte{0})
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func WriteManifest(dir string, files []string, hash string) error {
	m := Manifest{Hash: hash, Files: files}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(ManifestPath(dir), data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// WriteManifestFromMap computes the hash from a files map, then writes
// the manifest. Preferred over calling ManifestHash + WriteManifest
// separately — avoids duplicate path extraction.
func WriteManifestFromMap(dir string, files map[string][]byte) error {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	hash := ManifestHash(files)
	return WriteManifest(dir, paths, hash)
}

func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(ManifestPath(dir))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}
