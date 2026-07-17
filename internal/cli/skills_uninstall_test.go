package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/udit-001/waypoint/internal/skills"
)

// Manifest-driven deletion: only files listed in the manifest are removed,
// then the manifest, skillDir, and baseDir are cleaned up.
func TestUninstallSkills_WithManifest(t *testing.T) {
	base := t.TempDir()
	skillDir := filepath.Join(base, "waypoint")

	files := []string{"SKILL.md", "references/scraping.md"}
	for _, f := range files {
		p := filepath.Join(skillDir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := skills.WriteManifest(skillDir, files, "sha256:fake"); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if err := uninstallSkills(base); err != nil {
		t.Fatalf("uninstallSkills: %v", err)
	}

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skillDir should be removed, got err=%v", err)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Errorf("base should be removed, got err=%v", err)
	}
}

// Fallback path: when no manifest exists, the embedded file list is used.
func TestUninstallSkills_FallbackNoManifest(t *testing.T) {
	base := t.TempDir()
	skillDir := filepath.Join(base, "waypoint")

	embedded, err := skillFilesMap("waypoint")
	if err != nil {
		t.Fatalf("skillFilesMap: %v", err)
	}
	for rel, content := range embedded {
		p := filepath.Join(skillDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := uninstallSkills(base); err != nil {
		t.Fatalf("uninstallSkills: %v", err)
	}

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skillDir should be removed, got err=%v", err)
	}
}

// A non-existent baseDir is a no-op, not an error.
func TestUninstallSkills_NonExistentBaseDir(t *testing.T) {
	base := filepath.Join(t.TempDir(), "does-not-exist")
	if err := uninstallSkills(base); err != nil {
		t.Errorf("uninstallSkills on non-existent dir should not error, got: %v", err)
	}
}
