//go:build darwin

package cli

import (
	"github.com/kardianos/service"
)

func init() {
	newOSService = newDarwinService
}

// darwinService delegates to a launchd LaunchAgent written by kardianos.
type darwinService struct {
	svc service.Service
}

func newDarwinService(opts serviceOptions) (serviceController, error) {
	svc, err := service.New(&noopProgram{}, &service.Config{
		Name:        opts.Name,
		DisplayName: opts.Display,
		Description: opts.Desc,
		Arguments:   opts.Args,
		Option: service.KeyValue{
			// A per-user agent: no root, loads at login.
			"UserService": true,
		},
	})
	if err != nil {
		return nil, formatError("create service", err)
	}
	return &darwinService{svc: svc}, nil
}

func (d *darwinService) Install() error { return d.svc.Install() }

// Uninstall only unregisters; stopping is the command's job (see
// serviceUninstallCmd), so both platforms share one ordering.
func (d *darwinService) Uninstall() error { return d.svc.Uninstall() }

func (d *darwinService) Start() error { return d.svc.Start() }
func (d *darwinService) Stop() error  { return d.svc.Stop() }

func (d *darwinService) Status() (ServiceState, error) { return serviceStateOf(d.svc) }

// noopProgram satisfies service.Interface but is never called: the agent runs
// `waypoint start --daemon`, so the library's Run() loop is unused.
type noopProgram struct{}

func (p *noopProgram) Start(s service.Service) error { return nil }
func (p *noopProgram) Stop(s service.Service) error  { return nil }
