package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfigDir overrides the internal configDir seam to point at a
// temp directory, returning a cleanup function. This avoids t.Setenv().
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := configDir
	configDir = func() string { return dir }
	t.Cleanup(func() { configDir = orig })
	return dir
}

func TestConfigDir(t *testing.T) {
	dir := withTempConfigDir(t)
	got := ConfigDir()
	if got != dir {
		t.Errorf("ConfigDir() = %q, want %q", got, dir)
	}
}

func TestConfigPath(t *testing.T) {
	withTempConfigDir(t)
	got := ConfigPath()
	if filepath.Base(got) != "config.toml" {
		t.Errorf("ConfigPath() = %q, want base 'config.toml'", got)
	}
}

func TestPidPath(t *testing.T) {
	withTempConfigDir(t)
	got := PidPath()
	if filepath.Base(got) != "server.pid" {
		t.Errorf("PidPath() = %q, want base 'server.pid'", got)
	}
}

func TestDefaultDataDir(t *testing.T) {
	got := DefaultDataDir()
	if filepath.Base(got) != ".waypoint" {
		t.Errorf("DefaultDataDir() = %q, want base '.waypoint'", got)
	}
}

func TestDBPath(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "with config",
			cfg:  &Config{DataDir: "/tmp/data"},
			want: filepath.Join("/tmp/data", "waypoint.db"),
		},
		{
			name: "nil config",
			cfg:  nil,
			want: filepath.Join(DefaultDataDir(), "waypoint.db"),
		},
		{
			name: "empty data dir",
			cfg:  &Config{DataDir: ""},
			want: filepath.Join(DefaultDataDir(), "waypoint.db"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DBPath(tc.cfg)
			if got != tc.want {
				t.Errorf("DBPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadReturnsNilWhenFileMissing(t *testing.T) {
	withTempConfigDir(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg != nil {
		t.Fatalf("Load() = %v, want nil", cfg)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	withTempConfigDir(t)

	original := &Config{
		DataDir: "/custom/data",
		Port:    9999,
	}
	if err := Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(ConfigPath()); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil config after Save")
	}
	if loaded.DataDir != original.DataDir {
		t.Errorf("DataDir = %q, want %q", loaded.DataDir, original.DataDir)
	}
	if loaded.Port != original.Port {
		t.Errorf("Port = %d, want %d", loaded.Port, original.Port)
	}
}

func TestLoadFillsDefaults(t *testing.T) {
	withTempConfigDir(t)

	// Save a config with empty fields
	if err := Save(&Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DataDir != DefaultDataDir() {
		t.Errorf("DataDir = %q, want %q (default)", cfg.DataDir, DefaultDataDir())
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d (default)", cfg.Port, DefaultPort)
	}
}

func TestSaveCreatesConfigDir(t *testing.T) {
	// Use a nested path that doesn't exist yet
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested", "deep")
	orig := configDir
	configDir = func() string { return nested }
	t.Cleanup(func() { configDir = orig })

	if err := Save(&Config{DataDir: "/x", Port: 1}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, "config.toml")); err != nil {
		t.Fatalf("config file not created in nested dir: %v", err)
	}
}
