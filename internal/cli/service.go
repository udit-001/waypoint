package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
)

// osService is the seam for OS service operations. Production uses
// newOSService() backed by kardianos/service; tests inject a fake.
var newOSService func(name, displayName, description string, args []string) (serviceController, error)

// serviceController abstracts the OS service lifecycle for testing.
type serviceController interface {
	Install() error
	Status() (string, error)
	Remove() error
}

// Service name constants — shared across all platforms.
const (
	svcName        = "com.waypoint.app"
	svcDisplayName = "Waypoint"
	svcDescription = "Keeps your job search running in the background"
)

// svcRunCmd is the arguments passed to the waypoint binary when run as a
// service. The --daemon flag tells start to skip browser-open and run
// silently (PID file, health check, and pid-path all work unchanged).
var svcRunCmd = []string{"start", "--daemon"}

// --- commands ---

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the Waypoint background service",
	Long: `Manage the Waypoint background service that keeps your
job search running automatically.

The service starts Waypoint when your computer boots and
keeps it running in the background.

Examples:
  waypoint service install
  waypoint service status
  waypoint service remove`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Waypoint background service",
	Long: `Install Waypoint as a background service that starts
automatically when you log in.

Uses your operating system's native service manager
(systemd, launchd, or Windows Service Manager) to register
the service. The service runs as your current user.

Examples:
  waypoint service install
  waypoint service install --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSService(svcName, svcDisplayName, svcDescription, svcRunCmd)
		if err != nil {
			return formatError("init service", err)
		}
		if err := svc.Install(); err != nil {
			return formatError("install service", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": "installed", "service": svcName})
			return nil
		}
		fmt.Println()
		fmt.Println("  ✓ Service installed: Waypoint will start when you log in.")
		fmt.Println()
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the Waypoint service status",
	Long: `Show whether the Waypoint background service is installed,
running, or stopped.

Status values:
  running   — service is active and the server is responding
  stopped   — service is installed but not running
  not found — service is not installed

Examples:
  waypoint service status
  waypoint service status --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSService(svcName, svcDisplayName, svcDescription, svcRunCmd)
		if err != nil {
			return formatError("init service", err)
		}
		status, err := svc.Status()
		if err != nil {
			return formatError("check service status", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": status, "service": svcName})
			return nil
		}
		fmt.Println()
		switch status {
		case "running":
			fmt.Println("  ● Waypoint service is running")
		case "stopped":
			fmt.Println("  ○ Waypoint service is stopped")
		default:
			fmt.Println("  · Waypoint service is not installed")
		}
		fmt.Println()
		return nil
	},
}

var serviceRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the Waypoint background service",
	Long: `Remove the Waypoint background service from your system.

The server process is stopped first (if running), then the
service is unregistered from your OS service manager.

Examples:
  waypoint service remove
  waypoint service remove --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSService(svcName, svcDisplayName, svcDescription, svcRunCmd)
		if err != nil {
			return formatError("init service", err)
		}
		if err := svc.Remove(); err != nil {
			return formatError("remove service", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": "removed", "service": svcName})
			return nil
		}
		fmt.Println()
		fmt.Println("  ✓ Service removed.")
		fmt.Println()
		return nil
	},
}

// offerServiceInstall prompts the user to install the Waypoint service
// at the end of `waypoint init`. The offer is skipped when the platform
// doesn't support user-context services (newOSService is nil).
func offerServiceInstall(cfg *config.Config) {
	if cfg.ServiceInstalled {
		return
	}
	if newOSService == nil {
		return
	}

	fmt.Println()
	fmt.Println("  Start Waypoint automatically when your computer starts?")
	fmt.Println()
	fmt.Println("    This keeps your job search running in the background —")
	fmt.Println("    new job matches and alerts reach you without opening anything.")
	fmt.Println()
	fmt.Print("    [Y] Start automatically   [n] No, I'll open it myself: ")
	if !promptDefaultYes() {
		return
	}

	svc, err := newOSService(svcName, svcDisplayName, svcDescription, svcRunCmd)
	if err != nil {
		fmt.Printf("    Service setup failed: %v\n", err)
		return
	}
	if err := svc.Install(); err != nil {
		fmt.Printf("    Service install failed: %v\n", err)
		return
	}
	cfg.ServiceInstalled = true
	fmt.Println()
	fmt.Println("    ✓ Waypoint will start when you log in.")
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceRemoveCmd)
}
