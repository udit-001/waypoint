package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
)

// Service name constants — shared across all platforms.
const (
	svcName        = "com.waypoint.app"
	svcDisplayName = "Waypoint"
	svcDescription = "Keeps your job search running in the background"
)

// svcRunCmd is the daemon command a service manager runs. It is unpinned
// (daemonFlags(0)): systemd and launchd re-run this on every start, so a
// pinned port would go stale after a config change.
var svcRunCmd = daemonFlags(0)

// ServiceState is the typed vocabulary every platform adapter reports, so the
// commands never branch on a manager's own status strings.
type ServiceState int

const (
	// ServiceNotFound: not registered on this machine.
	ServiceNotFound ServiceState = iota
	// ServiceStopped: registered but not serving.
	ServiceStopped
	// ServiceRunning: registered and answering.
	ServiceRunning
)

// String is the user-facing and JSON form of a state.
func (s ServiceState) String() string {
	switch s {
	case ServiceRunning:
		return "running"
	case ServiceStopped:
		return "stopped"
	default:
		return "not found"
	}
}

// serviceController is the per-platform seam: five verbs, each one a thing a
// service manager (or our own supervisor, on Windows) can actually do.
//
// Restart is deliberately absent. It is Stop followed by Start, composed once
// in the command below, so no adapter has to reimplement the pair.
type serviceController interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (ServiceState, error)
}

// serviceOptions is everything a platform adapter needs. It is one struct
// rather than a growing parameter list so a new platform can take what it
// needs without changing every caller.
type serviceOptions struct {
	Exe     string   // absolute path to the running binary
	Args    []string // daemon arguments the manager should run
	Name    string   // service id / launchd label / Run value
	Display string
	Desc    string
}

// newOSService builds the platform adapter. Tests inject a fake.
var newOSService func(serviceOptions) (serviceController, error)

// serviceActive reports whether a service manager has Waypoint registered
// *and* running. That is the condition that makes `waypoint stop` the wrong
// command: the manager would restart what we just killed. A service that is
// merely installed (stopped) does not own a hand-run server, so the normal
// stop path still applies.
func serviceActive() bool {
	svc, err := newOSServiceFor()
	if err != nil {
		return false
	}
	state, err := svc.Status()
	if err != nil {
		return false
	}
	return state == ServiceRunning
}

// configuredPort resolves the port the daemon binds: config file value, else
// the default. The config file is the single source — start.go resolves the
// same way — so the supervisor's health checks target the port the server
// actually uses.
func configuredPort() int {
	if cfg, err := config.Load(); err == nil && cfg != nil && cfg.Port > 0 {
		return cfg.Port
	}
	return config.DefaultPort
}

// serviceOptionsFor describes this machine's service.
func serviceOptionsFor() serviceOptions {
	return serviceOptions{
		Exe:     exePath(),
		Args:    svcRunCmd,
		Name:    svcName,
		Display: svcDisplayName,
		Desc:    svcDescription,
	}
}

// --- commands ---

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the Waypoint background service",
	Long: `Manage the service that keeps your job search running automatically.

Waypoint uses whatever the platform provides: a systemd user unit on Linux,
a launchd agent on macOS, and on Windows — where a per-user service needs
administrator rights — a per-user logon entry plus Waypoint's own supervisor,
which restarts the server if it crashes. None of these need admin.

Use 'waypoint service stop' rather than 'waypoint stop' for a service-managed
server: the supervisor would treat a killed server as a crash and restart it.

Examples:
  waypoint service install
  waypoint service status
  waypoint service restart`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Waypoint background service",
	Long: `Install Waypoint as a background service that starts automatically.

Uses your operating system's native mechanism (systemd user unit, launchd
agent, or a per-user logon entry on Windows). The service runs as your
current user and needs no administrator rights.

Examples:
  waypoint service install
  waypoint service install --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSServiceFor()
		if err != nil {
			return err
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

var serviceUninstallCmd = &cobra.Command{
	Use:     "uninstall",
	Aliases: []string{"remove"},
	Short:   "Uninstall the Waypoint background service",
	Long: `Remove the Waypoint background service from your system.

The running server is stopped first, then the service is unregistered.

Examples:
  waypoint service uninstall
  waypoint service uninstall --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSServiceFor()
		if err != nil {
			return err
		}
		// Stop first, then unregister — composed here for the same reason
		// restart is: the managers disagree (systemd's Uninstall only disables
		// the unit, leaving a running server holding its port; launchd's stops
		// it), so one ordering at this seam keeps the platforms alike. A failed
		// stop is not fatal — there may be nothing running, and unregistering
		// is the point.
		_ = svc.Stop()
		if err := svc.Uninstall(); err != nil {
			return formatError("uninstall service", err)
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

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSServiceFor()
		if err != nil {
			return err
		}
		if err := svc.Start(); err != nil {
			return formatError("start service", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": "started", "service": svcName})
			return nil
		}
		fmt.Println()
		fmt.Println("  ✓ Service started.")
		fmt.Println()
		return nil
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the service (without uninstalling it)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSServiceFor()
		if err != nil {
			return err
		}
		if err := svc.Stop(); err != nil {
			return formatError("stop service", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": "stopped", "service": svcName})
			return nil
		}
		fmt.Println()
		fmt.Println("  ✓ Service stopped.")
		fmt.Println()
		return nil
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSServiceFor()
		if err != nil {
			return err
		}
		// Stop then Start, composed once here rather than on every platform.
		if err := svc.Stop(); err != nil {
			return formatError("stop service", err)
		}
		if err := svc.Start(); err != nil {
			return formatError("start service", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": "restarted", "service": svcName})
			return nil
		}
		fmt.Println()
		fmt.Println("  ✓ Service restarted.")
		fmt.Println()
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the Waypoint service status",
	Long: `Show whether the Waypoint background service is registered and running.

Status values:
  running   — registered and the server is answering
  stopped   — registered but not running
  not found — not registered

Examples:
  waypoint service status
  waypoint service status --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := newOSServiceFor()
		if err != nil {
			return err
		}
		state, err := svc.Status()
		if err != nil {
			return formatError("check service status", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": state.String(), "service": svcName})
			return nil
		}
		fmt.Println()
		switch state {
		case ServiceRunning:
			fmt.Println("  ● Waypoint service is running")
		case ServiceStopped:
			fmt.Println("  ○ Waypoint service is stopped")
		default:
			fmt.Println("  · Waypoint service is not installed")
		}
		fmt.Println()
		return nil
	},
}

// newOSServiceFor builds the platform adapter, or reports that this build has
// none.
func newOSServiceFor() (serviceController, error) {
	if newOSService == nil {
		return nil, fmt.Errorf("no background service is available on this platform")
	}
	return newOSService(serviceOptionsFor())
}

// offerServiceInstall prompts the user to install the Waypoint service at the
// end of `waypoint init`.
func offerServiceInstall(cfg *config.Config) {
	if cfg.ServiceInstalled || newOSService == nil {
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

	svc, err := newOSServiceFor()
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
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceRunCmd)
	serviceCmd.AddCommand(serviceSuperviseCmd)
}
