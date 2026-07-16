package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the waypoint configuration stored as TOML in the OS config dir.
type Config struct {
	DataDir string `toml:"data_dir"`
	Port    int    `toml:"port"`
}

// DefaultPort is the default HTTP server port.
const DefaultPort = 8080

// configDir is the internal seam for path resolution. Production uses
// defaultConfigDir (which calls os.UserConfigDir). Tests override this
// with a function returning t.TempDir() — no env var manipulation needed.
var configDir = defaultConfigDir

func defaultConfigDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(d, "waypoint")
}

func homeDir() string {
	d, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return d
}

// ConfigDir returns the OS-specific config directory for waypoint.
func ConfigDir() string {
	return configDir()
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// PidPath returns the path to the PID file.
func PidPath() string {
	return filepath.Join(ConfigDir(), "server.pid")
}

// DefaultDataDir returns the default data directory (~/.waypoint/).
func DefaultDataDir() string {
	return filepath.Join(homeDir(), ".waypoint")
}

// DBPath returns the database path for a given config.
func DBPath(cfg *Config) string {
	if cfg != nil && cfg.DataDir != "" {
		return filepath.Join(cfg.DataDir, "waypoint.db")
	}
	return filepath.Join(DefaultDataDir(), "waypoint.db")
}

// Load reads and parses the config file. Returns nil, nil if the file
// does not exist (not an error). Fills in defaults for empty fields.
func Load() (*Config, error) {
	p := ConfigPath()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir()
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the config dir if needed.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	p := ConfigPath()
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", p, err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

// SetConfigDirForTesting overrides the config directory for tests.
// Returns a cleanup function to restore the original.
func SetConfigDirForTesting(dir string) func() {
	orig := configDir
	configDir = func() string { return dir }
	return func() { configDir = orig }
}
