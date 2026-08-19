//go:build windows

package cli

import (
	"fmt"
)

func init() {
	newOSService = newWindowsService
}

type windowsService struct{}

func newWindowsService(name, displayName, description string, args []string) (serviceController, error) {
	return nil, fmt.Errorf("user-context services are not yet supported on Windows\n\n  Use 'waypoint start --background' to run the server in the background.\n  It will stay running until you log out or run 'waypoint stop'.")
}

func (w *windowsService) Install() error {
	return fmt.Errorf("not implemented on Windows")
}

func (w *windowsService) Status() (string, error) {
	return "", fmt.Errorf("not implemented on Windows")
}

func (w *windowsService) Remove() error {
	return fmt.Errorf("not implemented on Windows")
}
