package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSkillInstalled(t *testing.T) {
	base := t.TempDir()

	if isSkillInstalled(base) {
		t.Error("expected false when waypoint/SKILL.md absent")
	}

	skillDir := filepath.Join(base, "waypoint")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !isSkillInstalled(base) {
		t.Error("expected true when waypoint/SKILL.md present")
	}
}

func TestSkillFilesMap(t *testing.T) {
	files, err := skillFilesMap("waypoint")
	if err != nil {
		t.Fatalf("skillFilesMap: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected non-empty file map")
	}
	if _, ok := files["SKILL.md"]; !ok {
		t.Error("expected SKILL.md in file map")
	}
	for name, content := range files {
		if len(content) == 0 {
			t.Errorf("file %q has empty content", name)
		}
	}
}

func TestFormatLocationLine(t *testing.T) {
	tests := []struct {
		name string
		loc  skillLocation
		want string
	}{
		{
			name: "basic",
			loc:  skillLocation{dir: "/foo/skills", status: "current", scope: "global"},
			want: "/foo/skills — current — global",
		},
		{
			name: "with readers",
			loc:  skillLocation{dir: "/foo/skills", status: "current", scope: "global", readers: []string{"opencode", "codex"}},
			want: "/foo/skills — current — global — opencode, codex",
		},
		{
			name: "unmanaged",
			loc:  skillLocation{dir: "/foo/skills", status: "unmanaged", scope: "project", unmanaged: true},
			want: "/foo/skills — unmanaged — project [unmanaged]",
		},
		{
			name: "readers and unmanaged",
			loc:  skillLocation{dir: "/foo/skills", status: "outdated", scope: "global", readers: []string{"claude"}, unmanaged: true},
			want: "/foo/skills — outdated — global — claude [unmanaged]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLocationLine(tt.loc)
			if got != tt.want {
				t.Errorf("formatLocationLine = %q, want %q", got, tt.want)
			}
			if !strings.Contains(got, tt.loc.dir) {
				t.Errorf("output missing dir %q", tt.loc.dir)
			}
		})
	}
}
