package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
)

// `waypoint init` must leave a usable database on disk. It is excluded from
// the root's PersistentPreRunE (there may be no database yet), so it has to
// resolve the path from the config it just ensured — otherwise the store
// opens an empty path, migrates an in-memory database, and discards it.

func TestInitCreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	restoreConfigDir := config.SetConfigDirForTesting(filepath.Join(dir, "config"))
	defer restoreConfigDir()

	// A settled config: the skill and service offers are already answered,
	// so init never reads stdin.
	dataDir := filepath.Join(dir, "data")
	cfg := &config.Config{
		DataDir:          dataDir,
		Port:             config.DefaultPort,
		SkillsInstalled:  true,
		ServiceInstalled: true,
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init: %v", err)
	}

	dbPath := filepath.Join(dataDir, "waypoint.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("init did not create %s: %v", dbPath, err)
	}

	// The file must be migrated, not just present: every table the CLI
	// reads has to be there.
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open created database: %v", err)
	}
	defer store.Close()

	if _, err := store.GetJobs(); err != nil {
		t.Errorf("jobs table unreadable: %v", err)
	}
	if _, err := store.ListRunLogs(1); err != nil {
		t.Errorf("autopilot_runs unreadable: %v", err)
	}
}

// A second init on an initialized database must refuse to overwrite it.
func TestInitRefusesExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	restoreConfigDir := config.SetConfigDirForTesting(filepath.Join(dir, "config"))
	defer restoreConfigDir()

	dataDir := filepath.Join(dir, "data")
	if err := config.Save(&config.Config{
		DataDir:          dataDir,
		Port:             config.DefaultPort,
		SkillsInstalled:  true,
		ServiceInstalled: true,
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("first init: %v", err)
	}
	dbPath := filepath.Join(dataDir, "waypoint.db")

	// A row that a re-init must not wipe.
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.InsertJob(db.Job{Company: "Acme", Position: "Engineer"}); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	store.Close()

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("second init: %v", err)
	}

	store, err = db.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store.Close()
	jobs, err := store.GetJobs()
	if err != nil {
		t.Fatalf("GetJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("jobs = %d, want the seeded job to survive a re-init", len(jobs))
	}
}
