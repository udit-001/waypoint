//go:build linux

package cli

import (
	"fmt"
	"strings"

	"github.com/kardianos/service"
)

func init() {
	newOSService = newLinuxService
}

type linuxService struct {
	svc service.Service
}

func newLinuxService(name, displayName, description string, args []string) (serviceController, error) {
	cfg := &service.Config{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Arguments:   args,
		Option: service.KeyValue{
			"UserService": true,
			"Restart":     "always",
		},
	}

	// Stub interface — waypoint doesn't implement service.Interface because
	// it manages its own lifecycle via PID file + health check. The library
	// only needs the Interface for Run(); Install/Status/Uninstall never
	// call it.
	svc, err := service.New(&noopProgram{}, cfg)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}
	return &linuxService{svc: svc}, nil
}

func (l *linuxService) Install() error {
	return l.svc.Install()
}

func (l *linuxService) Status() (string, error) {
	status, err := l.svc.Status()
	if err != nil {
		if strings.Contains(err.Error(), "not installed") {
			return "not found", nil
		}
		return "", err
	}
	switch status {
	case service.StatusRunning:
		return "running", nil
	case service.StatusStopped:
		return "stopped", nil
	default:
		return "unknown", nil
	}
}

func (l *linuxService) Remove() error {
	// Try to stop first — ignore errors (may not be running).
	_ = l.svc.Stop()
	return l.svc.Uninstall()
}

// noopProgram satisfies service.Interface but is never called — the CLI
// manages its own lifecycle via PID file + health check.
type noopProgram struct{}

func (p *noopProgram) Start(s service.Service) error { return nil }
func (p *noopProgram) Stop(s service.Service) error  { return nil }
