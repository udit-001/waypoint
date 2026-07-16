package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
)

// stopAction represents the action to take after validating a PID file.
type stopAction int

const (
	stopKill   stopAction = iota // PID is our server — kill it
	stopSkip                     // PID alive but not our server — skip kill, clean up
	stopStale                    // PID is dead — just clean up
	stopLegacy                   // Port=0 legacy file — fall back to kill
)

// decideStopAction determines what to do based on PID file info and health
// check results. Extracted as a pure function for testability.
func decideStopAction(info *pidInfo, serverRunning, pidAlive bool) stopAction {
	if info.Port == 0 {
		return stopLegacy
	}
	if serverRunning {
		return stopKill
	}
	if pidAlive {
		return stopSkip
	}
	return stopStale
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background web UI server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := readPidFile()
		if err != nil {
			// Missing PID file — not an error, just nothing to stop.
			fmt.Println()
			fmt.Println("  No running Waypoint server found")
			fmt.Println()
			return nil
		}

		// Health-check the server and the process before killing.
		serverRunning := info.Port > 0 && isServerRunning(info.Port)
		pidAlive := processAlive(info.PID)

		switch decideStopAction(info, serverRunning, pidAlive) {
		case stopKill, stopLegacy:
			// Confirmed our server (or legacy PID file) — kill it.
			if err := killProcess(info.PID); err != nil {
				return err
			}
			// Wait for the process to actually exit (up to 5 seconds)
			for i := 0; i < 50; i++ {
				if !processAlive(info.PID) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			_ = os.Remove(config.PidPath())
			fmt.Println()
			fmt.Printf("  Server (PID %d) stopped\n", info.PID)
			fmt.Println()

		case stopSkip:
			// PID is alive but not responding as our server — don't kill it.
			// It may be a different process (e.g. PID was reused by PostgreSQL).
			_ = os.Remove(config.PidPath())
			fmt.Println()
			fmt.Printf("  PID %d is alive but not responding on port %d — it may be a different process.\n", info.PID, info.Port)
			fmt.Println("  Stale PID file cleaned up.")
			fmt.Println()

		case stopStale:
			// Process is dead — just clean up the PID file.
			_ = os.Remove(config.PidPath())
			fmt.Println()
			fmt.Println("  No running server found (stale PID file cleaned up).")
			fmt.Println()
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
