//go:build linux

package cli

import (
	"github.com/kardianos/service"
)

func init() {
	newOSService = newLinuxService
}

// linuxService delegates to a systemd user unit written by kardianos. The
// library is worth its dependency here: it renders the unit, runs
// `systemctl --user enable --now`, and knows the unit path — on macOS the same
// seam reaches launchd.
type linuxService struct {
	svc service.Service
}

func newLinuxService(opts serviceOptions) (serviceController, error) {
	svc, err := service.New(&noopProgram{}, &service.Config{
		Name:        opts.Name,
		DisplayName: opts.Display,
		Description: opts.Desc,
		Arguments:   opts.Args,
		Option: service.KeyValue{
			// A user unit: no root, no system-wide footprint.
			"UserService": true,
			// Restart on crash. This is the one guarantee the Windows logon
			// entry cannot make, which is why Waypoint brings its own
			// supervisor there.
			"Restart": "always",
		},
	})
	if err != nil {
		return nil, formatError("create service", err)
	}
	return &linuxService{svc: svc}, nil
}

func (l *linuxService) Install() error { return l.svc.Install() }

// Uninstall stops the unit before removing it: `systemctl disable` does not
// stop a running unit, so removing the file alone would leave the server
// holding its port.
func (l *linuxService) Uninstall() error {
	_ = l.svc.Stop()
	return l.svc.Uninstall()
}

func (l *linuxService) Start() error { return l.svc.Start() }
func (l *linuxService) Stop() error  { return l.svc.Stop() }

func (l *linuxService) Status() (ServiceState, error) { return serviceStateOf(l.svc) }

// noopProgram satisfies service.Interface but is never called: the unit runs
// `waypoint start --daemon`, so the library's Run() loop is unused.
type noopProgram struct{}

func (p *noopProgram) Start(s service.Service) error { return nil }
func (p *noopProgram) Stop(s service.Service) error  { return nil }
