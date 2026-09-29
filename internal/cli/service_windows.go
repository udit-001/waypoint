//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/udit-001/waypoint/internal/config"
	"golang.org/x/sys/windows/registry"
)

func init() {
	newOSService = newWindowsService
}

// windowsService is the Windows arm: a per-user logon entry plus Waypoint's
// own supervisor.
//
// It is not a real Windows service, deliberately. Registering one needs
// administrator rights, and a service runs in session 0 — where it cannot
// open the user's browser, cannot post desktop notifications, and resolves
// %APPDATA% to a different profile, so it would read the wrong database. A
// per-user logon entry runs as the user, in the user's session, with no
// elevation, and the supervisor (internal/supervise) supplies the
// restart-on-crash behaviour that session 0 would have given us for free.
type windowsService struct{ opts serviceOptions }

func newWindowsService(opts serviceOptions) (serviceController, error) {
	if opts.Exe == "" {
		return nil, errors.New("cannot resolve the waypoint binary path")
	}
	return &windowsService{opts: opts}, nil
}

// Install writes the logon entry and starts the supervisor now, so the user
// does not have to log out to get a running service.
func (w *windowsService) Install() error {
	command, err := runValue(w.opts.Exe)
	if err != nil {
		return err
	}
	if err := writeRunValue(command); err != nil {
		return err
	}
	return w.Start()
}

// Uninstall only removes the logon entry; stopping is the command's job (see
// serviceUninstallCmd), so all three platforms share one ordering.
func (w *windowsService) Uninstall() error {
	return deleteRunValue()
}

// Start launches the supervisor and waits until its server answers. Any
// server left behind by a supervisor that was killed would make the new
// child fail to bind the port, so it is cleared first — but only when it can
// be verified. An unverifiable process holding the port is reported instead:
// killing it is the exact thing the identity check exists to prevent, and
// guessing here would trade a clear error for a possible wrong-process kill.
func (w *windowsService) Start() error {
	if runningSupervisor() == nil {
		outcome, info, err := stopServerByPidfile()
		if err != nil {
			return err
		}
		if (outcome == stopForeignPID || outcome == stopUnverifiable) && info != nil {
			return fmt.Errorf("port %d is held by process %d, which is not answering as Waypoint. End it with 'taskkill /PID %d /T /F' and run 'waypoint service start' again",
				configuredPort(), info.PID, info.PID)
		}
	}
	// A marker left over from an earlier stop would end this run at once.
	_ = os.Remove(config.StopRequestPath())
	if err := startSupervisorDetached(w.opts.Exe); err != nil {
		return err
	}
	if !waitForSupervisorReady(configuredPort(), supervisorStartTimeout) {
		return errors.New("the supervisor did not come up; see " + config.ServiceLogPath())
	}
	return nil
}

// Stop asks the supervisor to shut down; it stops its own server on the way
// out. With no supervisor there is only a hand-run server to stop.
func (w *windowsService) Stop() error {
	if runningSupervisor() == nil {
		_, _, err := stopServerByPidfile()
		return err
	}
	return stopSupervisorNow()
}

// Status answers honestly from two independent facts: the entry is on disk,
// and the supervisor is alive with a server answering on its port.
func (w *windowsService) Status() (ServiceState, error) {
	if _, ok := readRunValue(); !ok {
		return ServiceNotFound, nil
	}
	if runningSupervisor() != nil && isServerRunning(configuredPort()) {
		return ServiceRunning, nil
	}
	return ServiceStopped, nil
}

// --- the per-user logon entry ---

func writeRunValue(command string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return formatError("open the startup registry key", err)
	}
	defer k.Close()
	if err := k.SetStringValue(runValueName, command); err != nil {
		return formatError("write the startup entry", err)
	}
	return nil
}

func readRunValue() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	command, _, err := k.GetStringValue(runValueName)
	if err != nil {
		return "", false
	}
	return command, true
}

func deleteRunValue() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return nil // no key means no entry to remove
	}
	defer k.Close()
	if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return formatError("remove the startup entry", err)
	}
	return nil
}
