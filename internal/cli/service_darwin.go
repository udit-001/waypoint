//go:build darwin

package cli

import (
	"fmt"
	"strings"

	"github.com/kardianos/service"
)

func init() {
	newOSService = newDarwinService
}

type darwinService struct {
	svc service.Service
}

func newDarwinService(name, displayName, description string, args []string) (serviceController, error) {
	cfg := &service.Config{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Arguments:   args,
		Option: service.KeyValue{
			"UserService": true,
		},
	}

	svc, err := service.New(&noopProgram{}, cfg)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}
	return &darwinService{svc: svc}, nil
}

func (d *darwinService) Install() error {
	return d.svc.Install()
}

func (d *darwinService) Status() (string, error) {
	status, err := d.svc.Status()
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

func (d *darwinService) Remove() error {
	_ = d.svc.Stop()
	return d.svc.Uninstall()
}

// noopProgram satisfies service.Interface but is never called.
type noopProgram struct{}

func (p *noopProgram) Start(s service.Service) error { return nil }
func (p *noopProgram) Stop(s service.Service) error  { return nil }
