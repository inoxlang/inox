package nodeimpl

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/oklog/ulid/v2"
)

var (
	ErrAppAlreadyDeployed      = errors.New("application is already deployed")
	ErrAppAlreadyBeingDeployed = errors.New("application is already being deployed")
)

type ApplicationDeployment struct {
	appName node.ApplicationName
	ulid    ulid.ULID

	appModule *core.Module
	image     core.Image

	app       *Application
	nodeAgent *Agent
	finished  atomic.Bool
}

func (app *Application) PrepareDeployment(args node.ApplicationDeploymentParams) (node.ApplicationDeployment, error) {
	appMod := args.AppMod

	if appMod.ModuleKind != core.ApplicationModule {
		return nil, fmt.Errorf("module %s is of kind '%s' not 'application'", appMod.Name(), appMod.ModuleKind)
	}

	switch app.Status() {
	case node.DeployedApp:
		if !args.UpdateRunningApp {
			return nil, ErrAppAlreadyDeployed
		}
	}

	appName, err := node.ApplicationNameFrom(args.AppName)
	if err != nil {
		return nil, err
	}

	app.lock.Lock()
	defer app.lock.Unlock()

	return &ApplicationDeployment{
		ulid:    ulid.Make(),
		appName: appName,

		appModule: appMod,
		image:     args.BaseImg,

		nodeAgent: app.agent,
		app:       app,
	}, nil
}

func (d *ApplicationDeployment) Begin() error {
	switch d.app.Status() {
	case node.UndeployedApp:
	case node.DeployingApp:
	case node.DeployedApp:
	case node.GracefullyStoppingApp:
	case node.GracefullyStoppedApp:
	case node.ErroneouslyStoppedApp:
	}

	return nil
}
