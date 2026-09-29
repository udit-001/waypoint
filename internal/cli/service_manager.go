//go:build linux || darwin

package cli

import (
	"errors"

	"github.com/kardianos/service"
)

// serviceStateOf adapts a kardianos status to our typed state. The library
// reports "not installed" as an error, but for us it is an ordinary state the
// status command must branch on, not a failure.
func serviceStateOf(svc service.Service) (ServiceState, error) {
	st, err := svc.Status()
	if err != nil {
		if errors.Is(err, service.ErrNotInstalled) {
			return ServiceNotFound, nil
		}
		return ServiceNotFound, err
	}
	switch st {
	case service.StatusRunning:
		return ServiceRunning, nil
	case service.StatusStopped:
		return ServiceStopped, nil
	default:
		return ServiceStopped, nil
	}
}
